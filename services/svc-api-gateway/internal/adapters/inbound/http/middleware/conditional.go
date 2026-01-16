package middleware

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
	"strings"
)

const (
	headerETag        = "ETag"
	headerIfNoneMatch = "If-None-Match"
)

// BufferedResponseWriter captures the response body for ETag generation.
type BufferedResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	body        *bytes.Buffer
	wroteHeader bool
	flusher     http.Flusher
	hijacker    http.Hijacker
}

// NewBufferedResponseWriter creates a new buffered response writer.
func NewBufferedResponseWriter(w http.ResponseWriter) *BufferedResponseWriter {
	brw := &BufferedResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           &bytes.Buffer{},
	}

	if f, ok := w.(http.Flusher); ok {
		brw.flusher = f
	}

	if h, ok := w.(http.Hijacker); ok {
		brw.hijacker = h
	}

	return brw
}

func (w *BufferedResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}

	w.statusCode = code
	w.wroteHeader = true
}

func (w *BufferedResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	return w.body.Write(b)
}

func (w *BufferedResponseWriter) StatusCode() int {
	return w.statusCode
}

func (w *BufferedResponseWriter) Body() []byte {
	return w.body.Bytes()
}

func (w *BufferedResponseWriter) Flush() {
	if w.flusher != nil {
		w.flusher.Flush()
	}
}

func (w *BufferedResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.hijacker != nil {
		return w.hijacker.Hijack()
	}

	return nil, nil, http.ErrNotSupported
}

func (w *BufferedResponseWriter) FlushToClient() error {
	w.ResponseWriter.WriteHeader(w.statusCode)
	_, err := w.ResponseWriter.Write(w.body.Bytes())

	return err
}

// ConditionalGET returns a middleware that handles conditional GET requests.
// It generates ETags for responses and returns 304 Not Modified when appropriate.
func ConditionalGET(generator *ETagGenerator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				next.ServeHTTP(w, r)

				return
			}

			brw := NewBufferedResponseWriter(w)

			next.ServeHTTP(brw, r)

			if brw.StatusCode() >= 300 {
				_ = brw.FlushToClient()

				return
			}

			body := brw.Body()
			etag := generator.Generate(body)

			ifNoneMatch := r.Header.Get(headerIfNoneMatch)
			if ifNoneMatch != "" && etagMatches(ifNoneMatch, etag) {
				for key, values := range brw.Header() {
					for _, value := range values {
						w.Header().Add(key, value)
					}
				}
				w.Header().Set(headerETag, formatETag(etag))
				w.WriteHeader(http.StatusNotModified)

				return
			}

			for key, values := range brw.Header() {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			w.Header().Set(headerETag, formatETag(etag))
			w.WriteHeader(brw.StatusCode())
			_, _ = w.Write(body)
		})
	}
}

func etagMatches(ifNoneMatch, etag string) bool {
	if ifNoneMatch == "*" {
		return true
	}

	quotedETag := formatETag(etag)
	weakETag := "W/" + quotedETag

	values := strings.Split(ifNoneMatch, ",")
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == quotedETag || value == weakETag {
			return true
		}
	}

	return false
}

func formatETag(etag string) string {
	if strings.HasPrefix(etag, "\"") && strings.HasSuffix(etag, "\"") {
		return etag
	}

	return "\"" + etag + "\""
}
