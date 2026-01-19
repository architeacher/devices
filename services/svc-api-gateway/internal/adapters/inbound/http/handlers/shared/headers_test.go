package shared_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/handlers/shared"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/stretchr/testify/require"
)

func TestSetCacheHeaders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                 string
		status               ports.CacheStatus
		key                  string
		ttl                  time.Duration
		maxAge               uint
		staleWhileRevalidate uint
		expectedCacheStatus  string
		expectedCacheKey     string
		expectedCacheTTL     string
		expectedCacheControl string
	}{
		{
			name:                 "cache hit with full TTL",
			status:               ports.CacheStatusHit,
			key:                  "device:v1:123",
			ttl:                  5 * time.Minute,
			maxAge:               60,
			staleWhileRevalidate: 30,
			expectedCacheStatus:  "HIT",
			expectedCacheKey:     "device:v1:123",
			expectedCacheTTL:     "300",
			expectedCacheControl: "private, max-age=60, stale-while-revalidate=30",
		},
		{
			name:                 "cache miss without TTL",
			status:               ports.CacheStatusMiss,
			key:                  "device:v1:456",
			ttl:                  0,
			maxAge:               60,
			staleWhileRevalidate: 0,
			expectedCacheStatus:  "MISS",
			expectedCacheKey:     "device:v1:456",
			expectedCacheTTL:     "",
			expectedCacheControl: "private, max-age=60",
		},
		{
			name:                 "bypass without key",
			status:               ports.CacheStatusBypass,
			key:                  "",
			ttl:                  0,
			maxAge:               120,
			staleWhileRevalidate: 60,
			expectedCacheStatus:  "BYPASS",
			expectedCacheKey:     "",
			expectedCacheTTL:     "",
			expectedCacheControl: "private, max-age=120, stale-while-revalidate=60",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()

			shared.SetCacheHeaders(w, tc.status, tc.key, tc.ttl, tc.maxAge, tc.staleWhileRevalidate)

			require.Equal(t, tc.expectedCacheStatus, w.Header().Get(shared.HeaderCacheStatus))
			require.Equal(t, tc.expectedCacheKey, w.Header().Get(shared.HeaderCacheKey))
			require.Equal(t, tc.expectedCacheTTL, w.Header().Get(shared.HeaderCacheTTL))
			require.Equal(t, tc.expectedCacheControl, w.Header().Get(shared.HeaderCacheControl))
			require.Contains(t, w.Header().Get(shared.HeaderVary), shared.HeaderAccept)
			require.Contains(t, w.Header().Get(shared.HeaderVary), shared.HeaderAuthorization)
		})
	}
}

func TestSetLastModified(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testTime := time.Date(2026, 1, 13, 12, 0, 0, 0, time.UTC)

	shared.SetLastModified(w, testTime)

	require.Equal(t, "Tue, 13 Jan 2026 12:00:00 GMT", w.Header().Get(shared.HeaderLastModified))
}

func TestSetETagHeader(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		etag        string
		expectedVal string
	}{
		{
			name:        "simple etag",
			etag:        "abc123",
			expectedVal: "\"abc123\"",
		},
		{
			name:        "hash etag",
			etag:        "a1b2c3d4e5f6",
			expectedVal: "\"a1b2c3d4e5f6\"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			shared.SetETagHeader(w, tc.etag)
			require.Equal(t, tc.expectedVal, w.Header().Get(shared.HeaderETag))
		})
	}
}

func TestSetWeakETagHeader(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	shared.SetWeakETagHeader(w, "abc123")
	require.Equal(t, "W/\"abc123\"", w.Header().Get(shared.HeaderETag))
}

func TestGetIfNoneMatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		headerValue string
		expected    string
	}{
		{
			name:        "no header",
			headerValue: "",
			expected:    "",
		},
		{
			name:        "quoted etag",
			headerValue: "\"abc123\"",
			expected:    "\"abc123\"",
		},
		{
			name:        "weak etag",
			headerValue: "W/\"abc123\"",
			expected:    "W/\"abc123\"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.headerValue != "" {
				req.Header.Set(shared.HeaderIfNoneMatch, tc.headerValue)
			}

			result := shared.GetIfNoneMatch(req)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestETagMatches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		headerValue string
		etag        string
		expected    bool
	}{
		{
			name:        "no header",
			headerValue: "",
			etag:        "abc123",
			expected:    false,
		},
		{
			name:        "exact match quoted",
			headerValue: "\"abc123\"",
			etag:        "abc123",
			expected:    true,
		},
		{
			name:        "exact match weak",
			headerValue: "W/\"abc123\"",
			etag:        "abc123",
			expected:    true,
		},
		{
			name:        "wildcard match",
			headerValue: "*",
			etag:        "abc123",
			expected:    true,
		},
		{
			name:        "no match",
			headerValue: "\"def456\"",
			etag:        "abc123",
			expected:    false,
		},
		{
			name:        "partial match fails",
			headerValue: "\"abc\"",
			etag:        "abc123",
			expected:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.headerValue != "" {
				req.Header.Set(shared.HeaderIfNoneMatch, tc.headerValue)
			}

			result := shared.ETagMatches(req, tc.etag)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestSetCacheStatusHeader(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		status   ports.CacheStatus
		expected string
	}{
		{
			name:     "HIT status",
			status:   ports.CacheStatusHit,
			expected: "HIT",
		},
		{
			name:     "MISS status",
			status:   ports.CacheStatusMiss,
			expected: "MISS",
		},
		{
			name:     "BYPASS status",
			status:   ports.CacheStatusBypass,
			expected: "BYPASS",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			shared.SetCacheStatusHeader(w, tc.status)

			require.Equal(t, tc.expected, w.Header().Get(shared.HeaderCacheStatus))
		})
	}
}

func TestIsCacheBypassRequested(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		cacheControl string
		pragma       string
		expected     bool
	}{
		{
			name:         "no headers",
			cacheControl: "",
			pragma:       "",
			expected:     false,
		},
		{
			name:         "Cache-Control no-cache",
			cacheControl: "no-cache",
			pragma:       "",
			expected:     true,
		},
		{
			name:         "Pragma no-cache",
			cacheControl: "",
			pragma:       "no-cache",
			expected:     true,
		},
		{
			name:         "both no-cache headers",
			cacheControl: "no-cache",
			pragma:       "no-cache",
			expected:     true,
		},
		{
			name:         "Cache-Control max-age",
			cacheControl: "max-age=0",
			pragma:       "",
			expected:     false,
		},
		{
			name:         "Pragma something else",
			cacheControl: "",
			pragma:       "something",
			expected:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.cacheControl != "" {
				req.Header.Set(shared.HeaderCacheControl, tc.cacheControl)
			}

			if tc.pragma != "" {
				req.Header.Set("Pragma", tc.pragma)
			}

			result := shared.IsCacheBypassRequested(req)
			require.Equal(t, tc.expected, result)
		})
	}
}
