package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/middleware"
	"github.com/stretchr/testify/require"
)

func TestConditionalGET(t *testing.T) {
	t.Parallel()

	generator := middleware.NewETagGenerator()
	responseBody := `{"id":"123","name":"Test Device"}`
	expectedETag := generator.Generate([]byte(responseBody))

	cases := []struct {
		name               string
		method             string
		ifNoneMatch        string
		expectedStatus     int
		expectBodyInResp   bool
		expectETagInHeader bool
	}{
		{
			name:               "GET without If-None-Match returns full response",
			method:             http.MethodGet,
			ifNoneMatch:        "",
			expectedStatus:     http.StatusOK,
			expectBodyInResp:   true,
			expectETagInHeader: true,
		},
		{
			name:               "GET with matching ETag returns 304",
			method:             http.MethodGet,
			ifNoneMatch:        "\"" + expectedETag + "\"",
			expectedStatus:     http.StatusNotModified,
			expectBodyInResp:   false,
			expectETagInHeader: true,
		},
		{
			name:               "GET with matching weak ETag returns 304",
			method:             http.MethodGet,
			ifNoneMatch:        "W/\"" + expectedETag + "\"",
			expectedStatus:     http.StatusNotModified,
			expectBodyInResp:   false,
			expectETagInHeader: true,
		},
		{
			name:               "GET with wildcard returns 304",
			method:             http.MethodGet,
			ifNoneMatch:        "*",
			expectedStatus:     http.StatusNotModified,
			expectBodyInResp:   false,
			expectETagInHeader: true,
		},
		{
			name:               "GET with non-matching ETag returns full response",
			method:             http.MethodGet,
			ifNoneMatch:        "\"different-etag\"",
			expectedStatus:     http.StatusOK,
			expectBodyInResp:   true,
			expectETagInHeader: true,
		},
		{
			name:               "HEAD without If-None-Match returns 200 with no body",
			method:             http.MethodHead,
			ifNoneMatch:        "",
			expectedStatus:     http.StatusOK,
			expectBodyInResp:   false,
			expectETagInHeader: true,
		},
		{
			name:               "HEAD with matching ETag returns 304",
			method:             http.MethodHead,
			ifNoneMatch:        "\"" + expectedETag + "\"",
			expectedStatus:     http.StatusNotModified,
			expectBodyInResp:   false,
			expectETagInHeader: true,
		},
		{
			name:               "POST bypasses conditional middleware",
			method:             http.MethodPost,
			ifNoneMatch:        "\"" + expectedETag + "\"",
			expectedStatus:     http.StatusOK,
			expectBodyInResp:   true,
			expectETagInHeader: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(responseBody))
			})

			middleware := middleware.ConditionalGET(generator)
			req := httptest.NewRequest(tc.method, "/test", nil)
			if tc.ifNoneMatch != "" {
				req.Header.Set("If-None-Match", tc.ifNoneMatch)
			}
			rec := httptest.NewRecorder()

			middleware(handler).ServeHTTP(rec, req)

			require.Equal(t, tc.expectedStatus, rec.Code)

			if tc.expectETagInHeader {
				etag := rec.Header().Get("ETag")
				require.NotEmpty(t, etag, "Expected ETag header")
				require.Equal(t, "\""+expectedETag+"\"", etag)
			}

			if tc.expectBodyInResp && tc.method != http.MethodHead {
				require.Equal(t, responseBody, rec.Body.String())
			} else if tc.expectedStatus == http.StatusNotModified {
				require.Empty(t, rec.Body.String(), "304 response should not have body")
			}
		})
	}
}

func TestConditionalGET_ErrorResponse(t *testing.T) {
	t.Parallel()

	generator := middleware.NewETagGenerator()
	errorBody := `{"error":"not found"}`

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(errorBody))
	})

	middleware := middleware.ConditionalGET(generator)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("If-None-Match", "*")
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, errorBody, rec.Body.String())
}

func TestConditionalGET_MultipleETags(t *testing.T) {
	t.Parallel()

	generator := middleware.NewETagGenerator()
	responseBody := `{"id":"123"}`
	expectedETag := generator.Generate([]byte(responseBody))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseBody))
	})

	cases := []struct {
		name           string
		ifNoneMatch    string
		expectedStatus int
	}{
		{
			name:           "matching ETag in list returns 304",
			ifNoneMatch:    "\"other-etag\", \"" + expectedETag + "\", \"another-etag\"",
			expectedStatus: http.StatusNotModified,
		},
		{
			name:           "no matching ETag in list returns 200",
			ifNoneMatch:    "\"etag1\", \"etag2\", \"etag3\"",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mw := middleware.ConditionalGET(generator)
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("If-None-Match", tc.ifNoneMatch)
			rec := httptest.NewRecorder()

			mw(handler).ServeHTTP(rec, req)

			require.Equal(t, tc.expectedStatus, rec.Code)
		})
	}
}

func TestConditionalGET_PreservesHeaders(t *testing.T) {
	t.Parallel()

	generator := middleware.NewETagGenerator()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom-Header", "custom-value")
		w.Header().Set("Cache-Control", "private, max-age=60")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"test":"data"}`))
	})

	mw := middleware.ConditionalGET(generator)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	mw(handler).ServeHTTP(rec, req)

	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.Equal(t, "custom-value", rec.Header().Get("X-Custom-Header"))
	require.Equal(t, "private, max-age=60", rec.Header().Get("Cache-Control"))
}

func TestBufferedResponseWriter(t *testing.T) {
	t.Parallel()

	t.Run("captures status code", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		brw := middleware.NewBufferedResponseWriter(rec)

		brw.WriteHeader(http.StatusCreated)

		require.Equal(t, http.StatusCreated, brw.StatusCode())
	})

	t.Run("captures body", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		brw := middleware.NewBufferedResponseWriter(rec)

		_, err := brw.Write([]byte("test body"))
		require.NoError(t, err)

		require.Equal(t, []byte("test body"), brw.Body())
	})

	t.Run("FlushToClient writes to underlying writer", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		brw := middleware.NewBufferedResponseWriter(rec)

		brw.WriteHeader(http.StatusCreated)
		_, _ = brw.Write([]byte("test body"))

		err := brw.FlushToClient()
		require.NoError(t, err)

		require.Equal(t, http.StatusCreated, rec.Code)
		require.Equal(t, "test body", rec.Body.String())
	})
}
