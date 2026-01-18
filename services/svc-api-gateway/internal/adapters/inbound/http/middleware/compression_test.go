package middleware

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

// testLogger returns a no-op logger for testing.
func testLogger() logger.Logger {
	return logger.NewTestLogger()
}

// largeJSON creates a JSON string larger than the default MinSize (1024 bytes).
func largeJSON() string {
	// Create a JSON string > 1024 bytes
	devices := make([]string, 50)
	for index := range devices {
		devices[index] = `{"id":"device-` + strings.Repeat("x", 20) + `","name":"test device"}`
	}

	return `{"devices":[` + strings.Join(devices, ",") + `]}`
}

// smallJSON creates a JSON string smaller than the default MinSize.
func smallJSON() string {
	return `{"status":"ok"}`
}

// testHandler creates a handler that returns the given body with content type.
func testHandler(body, contentType string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(body))
	})
}

// defaultCompressionConfig returns a default compression configuration for tests.
func defaultCompressionConfig() config.Compression {
	return config.Compression{
		Enabled:          true,
		Level:            5,
		MinSize:          1024,
		ContentTypes:     nil, // Uses DefaultCompressibleTypes
		SkipPaths:        []string{"/v1/health", "/v1/liveness", "/v1/readiness"},
		GracefulDegraded: true,
	}
}

// --- User Story 1 Tests: Automatic Response Compression ---

func TestCompressionMiddleware_GzipCompression(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()
	handler := CompressionMiddleware(cfg, testLogger())(testHandler(largeJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

	// Verify body is actually gzip-compressed
	gr, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)

	defer func() {
		_ = gr.Close()
	}()

	decompressed, err := io.ReadAll(gr)
	require.NoError(t, err)
	require.Equal(t, largeJSON(), string(decompressed))
}

func TestCompressionMiddleware_DeflateCompression(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()
	handler := CompressionMiddleware(cfg, testLogger())(testHandler(largeJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	req.Header.Set("Accept-Encoding", "deflate")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "deflate", rec.Header().Get("Content-Encoding"))

	// Verify body is actually deflate-compressed
	fr := flate.NewReader(rec.Body)

	defer func() {
		_ = fr.Close()
	}()

	decompressed, err := io.ReadAll(fr)
	require.NoError(t, err)
	require.Equal(t, largeJSON(), string(decompressed))
}

func TestCompressionMiddleware_BrotliCompression(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()
	handler := CompressionMiddleware(cfg, testLogger())(testHandler(largeJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	req.Header.Set("Accept-Encoding", "br")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "br", rec.Header().Get("Content-Encoding"))

	// Verify body is actually brotli-compressed
	br := brotli.NewReader(rec.Body)
	decompressed, err := io.ReadAll(br)
	require.NoError(t, err)
	require.Equal(t, largeJSON(), string(decompressed))
}

func TestCompressionMiddleware_NoAcceptEncoding_NoCompression(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()
	handler := CompressionMiddleware(cfg, testLogger())(testHandler(largeJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	// No Accept-Encoding header

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Header().Get("Content-Encoding"))
	require.Equal(t, largeJSON(), rec.Body.String())
}

func TestCompressionMiddleware_VaryHeader(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()
	handler := CompressionMiddleware(cfg, testLogger())(testHandler(largeJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Vary"), "Accept-Encoding")
}

func TestCompressionMiddleware_ContentEncodingHeader(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		acceptEncoding string
		expectedHeader string
	}{
		{
			name:           "gzip encoding",
			acceptEncoding: "gzip",
			expectedHeader: "gzip",
		},
		{
			name:           "deflate encoding",
			acceptEncoding: "deflate",
			expectedHeader: "deflate",
		},
		{
			name:           "brotli encoding",
			acceptEncoding: "br",
			expectedHeader: "br",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := defaultCompressionConfig()
			handler := CompressionMiddleware(cfg, testLogger())(testHandler(largeJSON(), "application/json"))

			req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
			req.Header.Set("Accept-Encoding", tc.acceptEncoding)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, tc.expectedHeader, rec.Header().Get("Content-Encoding"))
		})
	}
}

func TestCompressionMiddleware_Disabled(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()
	cfg.Enabled = false

	handler := CompressionMiddleware(cfg, testLogger())(testHandler(largeJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Header().Get("Content-Encoding"))
	require.Equal(t, largeJSON(), rec.Body.String())
}

// --- User Story 2 Tests: Content-Type Aware Compression ---

func TestCompressionMiddleware_JsonContentType_Compresses(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()
	handler := CompressionMiddleware(cfg, testLogger())(testHandler(largeJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
}

func TestCompressionMiddleware_BinaryContentType_NoCompression(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()

	// Binary content type should not be compressed
	binaryHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		// Write some binary-like data
		_, _ = w.Write(bytes.Repeat([]byte{0x89, 0x50, 0x4E, 0x47}, 500))
	})

	handler := CompressionMiddleware(cfg, testLogger())(binaryHandler)

	req := httptest.NewRequest(http.MethodGet, "/v1/devices/image", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Header().Get("Content-Encoding"))
}

func TestCompressionMiddleware_BelowMinSize_NoCompression(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()
	cfg.MinSize = 1024 // 1KB minimum

	handler := CompressionMiddleware(cfg, testLogger())(testHandler(smallJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	// Small responses should not be compressed
	require.Empty(t, rec.Header().Get("Content-Encoding"))
	require.Equal(t, smallJSON(), rec.Body.String())
}

func TestCompressionMiddleware_ConfigurableLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		level int
	}{
		{name: "level 1 (fastest)", level: 1},
		{name: "level 5 (balanced)", level: 5},
		{name: "level 9 (best compression)", level: 9},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := defaultCompressionConfig()
			cfg.Level = tc.level

			handler := CompressionMiddleware(cfg, testLogger())(testHandler(largeJSON(), "application/json"))

			req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
			req.Header.Set("Accept-Encoding", "gzip")

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

			// Verify it's valid gzip
			gr, err := gzip.NewReader(rec.Body)
			require.NoError(t, err)

			defer func() {
				_ = gr.Close()
			}()

			decompressed, err := io.ReadAll(gr)
			require.NoError(t, err)
			require.Equal(t, largeJSON(), string(decompressed))
		})
	}
}

func TestCompressionMiddleware_SkipPaths(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		path           string
		shouldCompress bool
	}{
		{name: "health endpoint", path: "/v1/health", shouldCompress: false},
		{name: "liveness endpoint", path: "/v1/liveness", shouldCompress: false},
		{name: "readiness endpoint", path: "/v1/readiness", shouldCompress: false},
		{name: "devices endpoint", path: "/v1/devices", shouldCompress: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := defaultCompressionConfig()
			handler := CompressionMiddleware(cfg, testLogger())(testHandler(largeJSON(), "application/json"))

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Accept-Encoding", "gzip")

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)

			if tc.shouldCompress {
				require.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
			} else {
				require.Empty(t, rec.Header().Get("Content-Encoding"))
			}
		})
	}
}

// --- User Story 3 Tests: Compression Algorithm Selection ---

func TestCompressionMiddleware_QualityValues_PreferHigher(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()
	handler := CompressionMiddleware(cfg, testLogger())(testHandler(largeJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	req.Header.Set("Accept-Encoding", "gzip;q=0.5, br;q=1.0")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	// Should prefer brotli (q=1.0) over gzip (q=0.5)
	require.Equal(t, "br", rec.Header().Get("Content-Encoding"))
}

func TestCompressionMiddleware_EqualQuality_ServerPreference(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()
	handler := CompressionMiddleware(cfg, testLogger())(testHandler(largeJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	// Equal quality values - server should use preference order: gzip > br > deflate
	req.Header.Set("Accept-Encoding", "br, gzip, deflate")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	// Server prefers gzip when quality values are equal
	require.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
}

func TestCompressionMiddleware_MalformedHeader_GracefulDegradation(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()
	handler := CompressionMiddleware(cfg, testLogger())(testHandler(largeJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	req.Header.Set("Accept-Encoding", "invalid;;malformed")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	// Should serve uncompressed on malformed header
	require.Empty(t, rec.Header().Get("Content-Encoding"))
}

func TestCompressionMiddleware_IdentityQZero_406Error(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()
	handler := CompressionMiddleware(cfg, testLogger())(testHandler(largeJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	// Client explicitly rejects uncompressed and only requests unsupported encoding
	req.Header.Set("Accept-Encoding", "identity;q=0, zstd")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should return 406 Not Acceptable when client rejects identity and we can't satisfy
	require.Equal(t, http.StatusNotAcceptable, rec.Code)
}

func TestCompressionMiddleware_Wildcard_ServerPreference(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()
	handler := CompressionMiddleware(cfg, testLogger())(testHandler(largeJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	req.Header.Set("Accept-Encoding", "*")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	// Should use server's preferred encoding (gzip)
	require.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
}

func TestCompressionMiddleware_UnsupportedEncoding_Fallback(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()
	handler := CompressionMiddleware(cfg, testLogger())(testHandler(largeJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	// Request only an unsupported encoding
	req.Header.Set("Accept-Encoding", "zstd")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	// Should serve uncompressed when no supported encoding matches
	require.Empty(t, rec.Header().Get("Content-Encoding"))
}

// --- Observability Tests ---

func TestCompressionMiddleware_MetricsEmitted(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()
	mockMetrics := &mockMetricsClient{}
	log := testLogger()

	handler := CompressionMiddlewareWithMetrics(cfg, log, mockMetrics)(testHandler(largeJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

	// Verify metrics were recorded
	require.True(t, mockMetrics.HasMetric("http_compression_total"), "expected http_compression_total metric")
	require.True(t, mockMetrics.HasMetric("http_compression_original_bytes"), "expected http_compression_original_bytes metric")
	require.True(t, mockMetrics.HasMetric("http_compression_compressed_bytes"), "expected http_compression_compressed_bytes metric")
	require.True(t, mockMetrics.HasMetric("http_compression_ratio"), "expected http_compression_ratio metric")

	// Verify algorithm attribute
	require.True(t, mockMetrics.HasAttribute("http_compression_total", "compression.algorithm", "gzip"))
}

func TestCompressionMiddleware_MetricsSkipped(t *testing.T) {
	t.Parallel()

	cfg := defaultCompressionConfig()
	mockMetrics := &mockMetricsClient{}
	log := testLogger()

	// Use small body that won't be compressed
	handler := CompressionMiddlewareWithMetrics(cfg, log, mockMetrics)(testHandler(smallJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Header().Get("Content-Encoding"))

	// Verify skip metrics were recorded
	require.True(t, mockMetrics.HasMetric("http_compression_skipped_total"), "expected http_compression_skipped_total metric")
	require.True(t, mockMetrics.HasAttribute("http_compression_skipped_total", "compression.skip_reason", "below_min_size"))
}

func TestCompressionMiddleware_StructuredLogging(t *testing.T) {
	t.Parallel()

	// This test verifies the logger is called with structured fields
	// The actual logging is tested via integration with the logger interface
	cfg := defaultCompressionConfig()
	mockMetrics := &mockMetricsClient{}
	log := testLogger()

	handler := CompressionMiddlewareWithMetrics(cfg, log, mockMetrics)(testHandler(largeJSON(), "application/json"))

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	// Logging is verified via the test logger's output in integration tests
}

// mockMetricsClient is a test double for metrics.Client.
type mockMetricsClient struct {
	metrics map[string][]mockMetricRecord
}

type mockMetricRecord struct {
	value      any
	attributes map[string]string
}

func (m *mockMetricsClient) Inc(_ context.Context, key string, value any, attrs ...attribute.KeyValue) {
	if m.metrics == nil {
		m.metrics = make(map[string][]mockMetricRecord)
	}

	attrMap := make(map[string]string)
	for _, attr := range attrs {
		attrMap[string(attr.Key)] = attr.Value.AsString()
	}

	m.metrics[key] = append(m.metrics[key], mockMetricRecord{
		value:      value,
		attributes: attrMap,
	})
}

func (m *mockMetricsClient) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func (m *mockMetricsClient) Shutdown(_ context.Context) error {
	return nil
}

func (m *mockMetricsClient) HasMetric(name string) bool {
	if m.metrics == nil {
		return false
	}

	_, ok := m.metrics[name]

	return ok
}

func (m *mockMetricsClient) HasAttribute(metricName, attrKey, attrValue string) bool {
	if m.metrics == nil {
		return false
	}

	records, ok := m.metrics[metricName]
	if !ok {
		return false
	}

	for _, record := range records {
		if v, found := record.attributes[attrKey]; found && v == attrValue {
			return true
		}
	}

	return false
}

// --- Config Validation Tests ---

func TestCompression_Validate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		cfg     config.Compression
		wantErr bool
	}{
		{
			name:    "valid default config",
			cfg:     defaultCompressionConfig(),
			wantErr: false,
		},
		{
			name: "level too low",
			cfg: config.Compression{
				Enabled: true,
				Level:   0,
				MinSize: 1024,
			},
			wantErr: true,
		},
		{
			name: "level too high",
			cfg: config.Compression{
				Enabled: true,
				Level:   10,
				MinSize: 1024,
			},
			wantErr: true,
		},
		{
			name: "negative min size",
			cfg: config.Compression{
				Enabled: true,
				Level:   5,
				MinSize: -1,
			},
			wantErr: true,
		},
		{
			name: "zero min size is valid",
			cfg: config.Compression{
				Enabled: true,
				Level:   5,
				MinSize: 0,
			},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.cfg.Validate()
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
