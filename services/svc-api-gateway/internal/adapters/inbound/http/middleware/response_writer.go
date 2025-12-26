package middleware

import (
	"bufio"
	"net"
	"net/http"
)

type FlushableResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten uint64
	wroteHeader  bool
	flusher      http.Flusher
	hijacker     http.Hijacker
	pusher       http.Pusher
}

func NewFlushableResponseWriter(w http.ResponseWriter) *FlushableResponseWriter {
	frw := &FlushableResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}

	if f, ok := w.(http.Flusher); ok {
		frw.flusher = f
	}

	if h, ok := w.(http.Hijacker); ok {
		frw.hijacker = h
	}

	if p, ok := w.(http.Pusher); ok {
		frw.pusher = p
	}

	return frw
}

func (w *FlushableResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}

	w.statusCode = code
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *FlushableResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += uint64(n)

	return n, err
}

func (w *FlushableResponseWriter) StatusCode() int {
	return w.statusCode
}

func (w *FlushableResponseWriter) BytesWritten() uint64 {
	return w.bytesWritten
}

func (w *FlushableResponseWriter) Flush() {
	if w.flusher != nil {
		w.flusher.Flush()
	}
}

func (w *FlushableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.hijacker != nil {
		return w.hijacker.Hijack()
	}

	return nil, nil, http.ErrNotSupported
}

func (w *FlushableResponseWriter) Push(target string, opts *http.PushOptions) error {
	if w.pusher != nil {
		return w.pusher.Push(target, opts)
	}

	return http.ErrNotSupported
}

func (w *FlushableResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
