package middleware

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"context"
	"io"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"go.opentelemetry.io/otel/attribute"
)

// DefaultCompressibleTypes lists MIME types that should be compressed by default.
// These are text-based types that typically benefit from compression.
var DefaultCompressibleTypes = []string{
	"application/json",
	"application/xml",
	"application/javascript",
	"application/problem+json",
	"text/html",
	"text/plain",
	"text/css",
	"text/javascript",
	"text/xml",
	"image/svg+xml",
}

// serverPreferenceOrder defines the server's algorithm preference when client
// quality values are equal. Per spec FR-018: gzip > brotli > deflate.
var serverPreferenceOrder = []string{"gzip", "br", "deflate"}

// Compression metrics constants.
const (
	compressionAlgorithmKey  = "compression.algorithm"
	compressionSkipReasonKey = "compression.skip_reason"

	httpCompressionTotal           = "http_compression_total"
	httpCompressionOriginalBytes   = "http_compression_original_bytes"
	httpCompressionCompressedBytes = "http_compression_compressed_bytes"
	httpCompressionRatio           = "http_compression_ratio"
	httpCompressionSkippedTotal    = "http_compression_skipped_total"
)

// Skip reasons for metrics.
const (
	skipReasonBelowMinSize    = "below_min_size"
	skipReasonNonCompressible = "non_compressible_type"
	skipReasonNoEncoding      = "no_accept_encoding"
	skipReasonSkippedPath     = "skipped_path"
)

// encoderPool pools compression writers to reduce GC pressure.
type encoderPool struct {
	gzip    sync.Pool
	deflate sync.Pool
	brotli  sync.Pool
}

var pools = encoderPool{
	gzip: sync.Pool{
		New: func() any {
			w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)

			return w
		},
	},
	deflate: sync.Pool{
		New: func() any {
			w, _ := flate.NewWriter(io.Discard, flate.DefaultCompression)

			return w
		},
	},
	brotli: sync.Pool{
		New: func() any {
			return brotli.NewWriterLevel(io.Discard, brotli.DefaultCompression)
		},
	},
}

// acceptEncoding represents a parsed Accept-Encoding value.
type acceptEncoding struct {
	encoding string
	quality  float64
}

// CompressionMiddleware creates a new compression middleware with support for gzip,
// deflate, and brotli compression. It respects Accept-Encoding quality values and
// applies the server's preference order when quality values are equal.
func CompressionMiddleware(cfg config.Compression, _ logger.Logger) func(http.Handler) http.Handler {
	if !cfg.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// Determine content types to compress
	contentTypes := cfg.ContentTypes
	if len(contentTypes) == 0 {
		contentTypes = DefaultCompressibleTypes
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip compression for configured paths (health checks, etc.)
			if shouldSkipPath(r.URL.Path, cfg.SkipPaths) {
				next.ServeHTTP(w, r)

				return
			}

			// Check if the client accepts any compression
			acceptHeader := r.Header.Get("Accept-Encoding")
			if acceptHeader == "" {
				next.ServeHTTP(w, r)

				return
			}

			// Parse Accept-Encoding header
			encodings := parseAcceptEncoding(acceptHeader)

			// Check for the identity;q=0 case (client rejects uncompressed)
			if rejectsIdentity(encodings) && !hasValidEncoding(encodings) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusNotAcceptable)
				_, _ = w.Write([]byte(`{"error":"not_acceptable","message":"No acceptable encoding available"}`))

				return
			}

			// Select the best encoding
			encoding := selectEncoding(encodings)
			if encoding == "" || encoding == "identity" {
				next.ServeHTTP(w, r)

				return
			}

			// Wrap response writer with compression
			cw := &compressResponseWriter{
				ResponseWriter: w,
				encoding:       encoding,
				level:          cfg.Level,
				minSize:        cfg.MinSize,
				contentTypes:   contentTypes,
			}

			defer func() { _ = cw.Close() }()

			w.Header().Set("Vary", "Accept-Encoding")
			next.ServeHTTP(cw, r)
		})
	}
}

// CompressionMiddlewareWithMetrics creates compression middleware with metrics and structured logging.
// This is the observability-enabled version that records compression statistics.
func CompressionMiddlewareWithMetrics(cfg config.Compression, log logger.Logger, metricsClient metrics.Client) func(http.Handler) http.Handler {
	if !cfg.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// Determine content types to compress
	contentTypes := cfg.ContentTypes
	if len(contentTypes) == 0 {
		contentTypes = DefaultCompressibleTypes
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Skip compression for configured paths (health checks, etc.)
			if shouldSkipPath(r.URL.Path, cfg.SkipPaths) {
				recordCompressionSkipped(ctx, metricsClient, skipReasonSkippedPath)
				next.ServeHTTP(w, r)

				return
			}

			// Check if client accepts any compression
			acceptHeader := r.Header.Get("Accept-Encoding")
			if acceptHeader == "" {
				recordCompressionSkipped(ctx, metricsClient, skipReasonNoEncoding)
				next.ServeHTTP(w, r)

				return
			}

			// Parse Accept-Encoding header
			encodings := parseAcceptEncoding(acceptHeader)

			// Check for identity;q=0 case (client rejects uncompressed)
			if rejectsIdentity(encodings) && !hasValidEncoding(encodings) {
				log.Warn().
					Str("accept_encoding", acceptHeader).
					Msg("client rejected all encodings, returning 406")

				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusNotAcceptable)
				_, _ = w.Write([]byte(`{"error":"not_acceptable","message":"No acceptable encoding available"}`))

				return
			}

			// Select the best encoding
			encoding := selectEncoding(encodings)
			if encoding == "" || encoding == "identity" {
				recordCompressionSkipped(ctx, metricsClient, skipReasonNoEncoding)
				next.ServeHTTP(w, r)

				return
			}

			// Wrap the response writer with compression and metrics
			cw := &compressResponseWriterWithMetrics{
				compressResponseWriter: compressResponseWriter{
					ResponseWriter: w,
					encoding:       encoding,
					level:          cfg.Level,
					minSize:        cfg.MinSize,
					contentTypes:   contentTypes,
				},
				ctx:           ctx,
				log:           log,
				metricsClient: metricsClient,
			}

			defer func() { _ = cw.Close() }()

			w.Header().Set("Vary", "Accept-Encoding")
			next.ServeHTTP(cw, r)
		})
	}
}

// recordCompressionSkipped records a skip metric with the given reason.
func recordCompressionSkipped(ctx context.Context, metricsClient metrics.Client, reason string) {
	if metricsClient == nil {
		return
	}

	metricsClient.Inc(
		ctx,
		httpCompressionSkippedTotal,
		int64(1),
		attribute.String(compressionSkipReasonKey, reason),
	)
}

// recordCompressionMetrics records compression success metrics.
func recordCompressionMetrics(ctx context.Context, metricsClient metrics.Client, algorithm string, originalSize, compressedSize int64) {
	if metricsClient == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String(compressionAlgorithmKey, algorithm),
	}

	metricsClient.Inc(ctx, httpCompressionTotal, int64(1), attrs...)
	metricsClient.Inc(ctx, httpCompressionOriginalBytes, originalSize, attrs...)
	metricsClient.Inc(ctx, httpCompressionCompressedBytes, compressedSize, attrs...)

	// Calculate and record ratio (as percentage, e.g., 0.65 = 65% of original)
	if originalSize > 0 {
		ratio := float64(compressedSize) / float64(originalSize)
		metricsClient.Inc(ctx, httpCompressionRatio, ratio, attrs...)
	}
}

// parseAcceptEncoding parses the Accept-Encoding header value.
func parseAcceptEncoding(header string) []acceptEncoding {
	var encodings []acceptEncoding

	parts := strings.Split(header, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		enc := acceptEncoding{quality: 1.0}

		// Split by semicolon to get encoding and optional quality
		subparts := strings.Split(part, ";")
		enc.encoding = strings.TrimSpace(subparts[0])

		// Parse quality value if present
		for _, sp := range subparts[1:] {
			sp = strings.TrimSpace(sp)
			if strings.HasPrefix(sp, "q=") {
				qVal := strings.TrimPrefix(sp, "q=")
				if q, err := strconv.ParseFloat(qVal, 64); err == nil {
					enc.quality = q
				}
			}
		}

		encodings = append(encodings, enc)
	}

	return encodings
}

// rejectsIdentity checks if the client explicitly rejects uncompressed content.
func rejectsIdentity(encodings []acceptEncoding) bool {
	for _, enc := range encodings {
		if enc.encoding == "identity" && enc.quality == 0 {
			return true
		}

		// Also check for *;q=0 which rejects all including identity
		if enc.encoding == "*" && enc.quality == 0 {
			return true
		}
	}

	return false
}

// hasValidEncoding checks if any supported encoding is acceptable.
func hasValidEncoding(encodings []acceptEncoding) bool {
	for _, enc := range encodings {
		if enc.quality > 0 {
			switch enc.encoding {
			case "gzip", "deflate", "br":
				return true
			case "*":
				// Wildcard with quality > 0 means any encoding is acceptable
				return true
			}
		}
	}

	return false
}

// selectEncoding selects the best encoding based on client preferences and server order.
func selectEncoding(encodings []acceptEncoding) string {
	// Check for wildcard first
	for _, enc := range encodings {
		if enc.encoding == "*" && enc.quality > 0 {
			// Use server's preferred encoding
			return serverPreferenceOrder[0]
		}
	}

	// Find highest quality among supported encodings
	type candidate struct {
		encoding string
		quality  float64
		priority int // Server preference (lower = more preferred)
	}

	var candidates []candidate

	for _, enc := range encodings {
		if enc.quality == 0 {
			continue
		}

		priority := -1

		for index, pref := range serverPreferenceOrder {
			if pref == enc.encoding {
				priority = index

				break
			}
		}

		if priority >= 0 {
			candidates = append(candidates, candidate{
				encoding: enc.encoding,
				quality:  enc.quality,
				priority: priority,
			})
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	// Sort by quality (desc), then by server preference (asc)
	slices.SortFunc(candidates, func(a, b candidate) int {
		if a.quality != b.quality {
			if a.quality > b.quality {
				return -1
			}

			return 1
		}
		// Equal quality - use server preference

		return a.priority - b.priority
	})

	return candidates[0].encoding
}

// shouldSkipPath checks if the given path should skip compression.
func shouldSkipPath(path string, skipPaths []string) bool {
	for _, skipPath := range skipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}

	return false
}

// compressResponseWriter wraps http.ResponseWriter to apply compression.
type compressResponseWriter struct {
	http.ResponseWriter
	encoding     string
	level        int
	minSize      int
	contentTypes []string

	writer        io.WriteCloser
	headerWritten bool
	buf           []byte
	shouldSkip    bool
}

func (w *compressResponseWriter) Write(b []byte) (int, error) {
	if w.shouldSkip {
		return w.ResponseWriter.Write(b)
	}

	// Buffer until we have enough data to decide
	if w.writer == nil && !w.headerWritten {
		w.buf = append(w.buf, b...)

		// Check if we have enough to decide
		if len(w.buf) >= w.minSize {
			w.initWriter()
		}

		return len(b), nil
	}

	if w.writer != nil {
		return w.writer.Write(b)
	}

	return w.ResponseWriter.Write(b)
}

func (w *compressResponseWriter) WriteHeader(statusCode int) {
	if w.headerWritten {
		return
	}

	// Check content type
	ct := w.Header().Get("Content-Type")
	if ct != "" && !w.isCompressible(ct) {
		w.shouldSkip = true
		w.ResponseWriter.WriteHeader(statusCode)
		w.headerWritten = true

		return
	}

	// For non-OK statuses that typically have no body, skip compression
	if statusCode == http.StatusNoContent || statusCode == http.StatusNotModified {
		w.shouldSkip = true
		w.ResponseWriter.WriteHeader(statusCode)
		w.headerWritten = true

		return
	}

	// Don't write header yet - wait for Write to decide on compression
	w.headerWritten = true

	// Store status code for later
	w.ResponseWriter.Header().Set("X-Pending-Status", strconv.Itoa(statusCode))
}

func (w *compressResponseWriter) initWriter() {
	// Determine final content type
	ct := w.Header().Get("Content-Type")
	if ct != "" && !w.isCompressible(ct) {
		w.shouldSkip = true
		w.flushBuffer()

		return
	}

	// Set compression headers
	w.Header().Set("Content-Encoding", w.encoding)
	w.Header().Del("Content-Length") // Will use chunked encoding

	// Get pending status code
	statusStr := w.Header().Get("X-Pending-Status")
	w.Header().Del("X-Pending-Status")

	statusCode := http.StatusOK
	if statusStr != "" {
		if s, err := strconv.Atoi(statusStr); err == nil {
			statusCode = s
		}
	}

	w.ResponseWriter.WriteHeader(statusCode)

	// Create encoder
	switch w.encoding {
	case "gzip":
		gw := pools.gzip.Get().(*gzip.Writer)
		gw.Reset(w.ResponseWriter)

		w.writer = &pooledGzipWriter{Writer: gw, pool: &pools.gzip}
	case "deflate":
		fw := pools.deflate.Get().(*flate.Writer)
		fw.Reset(w.ResponseWriter)

		w.writer = &pooledFlateWriter{Writer: fw, pool: &pools.deflate}
	case "br":
		bw := pools.brotli.Get().(*brotli.Writer)
		bw.Reset(w.ResponseWriter)

		w.writer = &pooledBrotliWriter{Writer: bw, pool: &pools.brotli}
	}

	// Write buffered data
	if len(w.buf) > 0 && w.writer != nil {
		_, _ = w.writer.Write(w.buf)
		w.buf = nil
	}
}

func (w *compressResponseWriter) flushBuffer() {
	statusStr := w.Header().Get("X-Pending-Status")
	w.Header().Del("X-Pending-Status")

	statusCode := http.StatusOK
	if statusStr != "" {
		if s, err := strconv.Atoi(statusStr); err == nil {
			statusCode = s
		}
	}

	w.ResponseWriter.WriteHeader(statusCode)

	if len(w.buf) > 0 {
		_, _ = w.ResponseWriter.Write(w.buf)
		w.buf = nil
	}
}

func (w *compressResponseWriter) isCompressible(contentType string) bool {
	// Extract media type without parameters
	ct := contentType
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = ct[:idx]
	}

	ct = strings.TrimSpace(strings.ToLower(ct))

	for _, allowed := range w.contentTypes {
		if strings.ToLower(allowed) == ct {
			return true
		}
	}

	return false
}

func (w *compressResponseWriter) Close() error {
	// If we never initiated compression, flush buffer
	if w.writer == nil && len(w.buf) > 0 {
		w.flushBuffer()
	}

	if w.writer != nil {
		return w.writer.Close()
	}

	return nil
}

// Implement http.Flusher
func (w *compressResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		if w.writer != nil {
			// Flush compression writer first
			if flusher, ok := w.writer.(interface{ Flush() error }); ok {
				_ = flusher.Flush()
			}
		}

		f.Flush()
	}
}

// Hijack Implement http.Hijacker
func (w *compressResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}

	return nil, nil, http.ErrNotSupported
}

// Pooled writers for memory efficiency
type pooledGzipWriter struct {
	*gzip.Writer
	pool *sync.Pool
}

func (w *pooledGzipWriter) Close() error {
	err := w.Writer.Close()
	w.pool.Put(w.Writer)

	return err
}

type pooledFlateWriter struct {
	*flate.Writer
	pool *sync.Pool
}

func (w *pooledFlateWriter) Close() error {
	err := w.Writer.Close()
	w.pool.Put(w.Writer)

	return err
}

type pooledBrotliWriter struct {
	*brotli.Writer
	pool *sync.Pool
}

func (w *pooledBrotliWriter) Close() error {
	err := w.Writer.Close()
	w.pool.Put(w.Writer)

	return err
}

// compressResponseWriterWithMetrics extends compressResponseWriter with metrics tracking.
type compressResponseWriterWithMetrics struct {
	compressResponseWriter
	ctx           context.Context
	log           logger.Logger
	metricsClient metrics.Client
	originalSize  int
}

func (w *compressResponseWriterWithMetrics) Write(b []byte) (int, error) {
	w.originalSize += len(b)

	return w.compressResponseWriter.Write(b)
}

func (w *compressResponseWriterWithMetrics) Close() error {
	// If we never initiated compression, flush buffer and record skip
	if w.writer == nil && len(w.buf) > 0 {
		w.flushBuffer()
		recordCompressionSkipped(w.ctx, w.metricsClient, skipReasonBelowMinSize)

		return nil
	}

	// If we skipped due to content type, record that
	if w.shouldSkip && w.writer == nil {
		recordCompressionSkipped(w.ctx, w.metricsClient, skipReasonNonCompressible)

		return nil
	}

	if w.writer != nil {
		err := w.writer.Close()

		// Calculate compressed size from the underlying response
		// We can estimate based on the compression ratio
		compressedSize := int64(w.originalSize)
		if rw, ok := w.ResponseWriter.(*FlushableResponseWriter); ok {
			compressedSize = int64(rw.BytesWritten())
		}

		// Record successful compression metrics
		recordCompressionMetrics(
			w.ctx,
			w.metricsClient,
			w.encoding,
			int64(w.originalSize),
			compressedSize,
		)

		// Log at DEBUG level for successful compression.
		if w.originalSize > 0 {
			ratio := float64(compressedSize) / float64(w.originalSize) * 100
			w.log.Debug().
				Str("compression_algorithm", w.encoding).
				Int("original_size", w.originalSize).
				Int64("compressed_size", compressedSize).
				Float64("compression_ratio", ratio).
				Msg("response compressed")
		}

		return err
	}

	return nil
}
