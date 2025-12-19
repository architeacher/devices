package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/middleware"
	"github.com/stretchr/testify/suite"
)

type SecurityHeadersTestSuite struct {
	suite.Suite
}

func TestSecurityHeadersTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(SecurityHeadersTestSuite))
}

func (s *SecurityHeadersTestSuite) TestSecurityHeaders() {
	s.T().Parallel()

	cases := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "X-Content-Type-Options",
			header:   "X-Content-Type-Options",
			expected: "nosniff",
		},
		{
			name:     "X-Frame-Options",
			header:   "X-Frame-Options",
			expected: "DENY",
		},
		{
			name:     "X-XSS-Protection",
			header:   "X-XSS-Protection",
			expected: "1; mode=block",
		},
		{
			name:     "Strict-Transport-Security",
			header:   "Strict-Transport-Security",
			expected: "max-age=31536000; includeSubDomains",
		},
		{
			name:     "Content-Security-Policy",
			header:   "Content-Security-Policy",
			expected: "default-src 'self'",
		},
		{
			name:     "Referrer-Policy",
			header:   "Referrer-Policy",
			expected: "strict-origin-when-cross-origin",
		},
		{
			name:     "Permissions-Policy",
			header:   "Permissions-Policy",
			expected: "camera=(), microphone=(), geolocation=()",
		},
		{
			name:     "API-Version",
			header:   "API-Version",
			expected: "v1",
		},
	}

	handler := middleware.SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, tc := range cases {
		s.Run(tc.name, func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			s.Require().Equal(tc.expected, rec.Header().Get(tc.header))
		})
	}
}

type CORSTestSuite struct {
	suite.Suite
}

func TestCORSTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(CORSTestSuite))
}

func (s *CORSTestSuite) TestCORS_AllowAll() {
	s.T().Parallel()

	handler := middleware.CORS([]string{"*"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	s.Require().Equal("https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	s.Require().NotEmpty(rec.Header().Get("Access-Control-Allow-Methods"))
	s.Require().NotEmpty(rec.Header().Get("Access-Control-Allow-Headers"))
}

func (s *CORSTestSuite) TestCORS_SpecificOrigin() {
	s.T().Parallel()

	handler := middleware.CORS([]string{"https://allowed.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		name          string
		origin        string
		expectAllowed bool
	}{
		{
			name:          "allowed origin",
			origin:        "https://allowed.com",
			expectAllowed: true,
		},
		{
			name:          "disallowed origin",
			origin:        "https://notallowed.com",
			expectAllowed: false,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if tc.expectAllowed {
				s.Require().Equal(tc.origin, rec.Header().Get("Access-Control-Allow-Origin"))
			} else {
				s.Require().Empty(rec.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func (s *CORSTestSuite) TestCORS_Preflight() {
	s.T().Parallel()

	handlerCalled := false
	handler := middleware.CORS([]string{"*"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	s.Require().Equal(http.StatusNoContent, rec.Code)
	s.Require().False(handlerCalled)
}

type RequestIDTestSuite struct {
	suite.Suite
}

func TestRequestIDTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(RequestIDTestSuite))
}

func (s *RequestIDTestSuite) TestRequestID_GeneratesNewID() {
	s.T().Parallel()

	var capturedCtx context.Context
	handler := middleware.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	s.Require().NotEmpty(rec.Header().Get(middleware.RequestIDHeader))
	s.Require().NotEmpty(middleware.GetRequestID(capturedCtx))
	s.Require().Equal(rec.Header().Get(middleware.RequestIDHeader), middleware.GetRequestID(capturedCtx))
}

func (s *RequestIDTestSuite) TestRequestID_UsesExistingID() {
	s.T().Parallel()

	existingID := "existing-request-id-123"
	var capturedCtx context.Context
	handler := middleware.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.RequestIDHeader, existingID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	s.Require().Equal(existingID, rec.Header().Get(middleware.RequestIDHeader))
	s.Require().Equal(existingID, middleware.GetRequestID(capturedCtx))
}

func (s *RequestIDTestSuite) TestGetRequestID_EmptyContext() {
	s.T().Parallel()

	ctx := context.Background()

	requestID := middleware.GetRequestID(ctx)

	s.Require().Empty(requestID)
}
