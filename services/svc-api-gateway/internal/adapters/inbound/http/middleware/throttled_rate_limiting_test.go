package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/middleware"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/stretchr/testify/suite"
	"github.com/throttled/throttled/v2/store/memstore"
)

type RateLimitingTestSuite struct {
	suite.Suite
	log    logger.Logger
	config config.ThrottledRateLimiting
}

func TestRateLimitingTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(RateLimitingTestSuite))
}

func (s *RateLimitingTestSuite) SetupTest() {
	s.log = logger.New("debug", "console")
	s.config = config.ThrottledRateLimiting{
		Enabled:            true,
		RequestsPerSecond:  10,
		BurstSize:          5,
		WindowDuration:     time.Minute,
		EnableIPLimiting:   true,
		EnableUserLimiting: true,
		CleanupInterval:    time.Minute,
		MaxKeys:            100,
		SkipPaths:          []string{"/health", "/liveness", "/readiness"},
	}
}

func (s *RateLimitingTestSuite) TestAllowsRequestsUnderLimit() {
	s.T().Parallel()

	store, err := memstore.NewCtx(100)
	s.Require().NoError(err)

	handler := middleware.ThrottledRateLimitingMiddleware(s.config, store, s.log)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
}

func (s *RateLimitingTestSuite) TestBlocksRequestsOverLimit() {
	s.T().Parallel()

	store, err := memstore.NewCtx(100)
	s.Require().NoError(err)

	cfg := s.config
	cfg.RequestsPerSecond = 1
	cfg.BurstSize = 0

	handler := middleware.ThrottledRateLimitingMiddleware(cfg, store, s.log)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// First request should succeed
	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.RemoteAddr = "192.168.1.2:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusOK, rec.Code)

	// Second immediate request should be rate limited
	req = httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.RemoteAddr = "192.168.1.2:12345"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusTooManyRequests, rec.Code)
}

func (s *RateLimitingTestSuite) TestSkipPathsBypassesRateLimiting() {
	s.T().Parallel()

	store, err := memstore.NewCtx(100)
	s.Require().NoError(err)

	cfg := s.config
	cfg.RequestsPerSecond = 1
	cfg.BurstSize = 0
	cfg.SkipPaths = []string{"/health"}

	handler := middleware.ThrottledRateLimitingMiddleware(cfg, store, s.log)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// Multiple requests to skip path should all succeed
	for index := 0; index < 5; index++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		s.Require().Equal(http.StatusOK, rec.Code, "request %d should not be rate limited", index+1)
	}
}

func (s *RateLimitingTestSuite) TestRFCHeadersAreSet() {
	s.T().Parallel()

	store, err := memstore.NewCtx(100)
	s.Require().NoError(err)

	handler := middleware.ThrottledRateLimitingMiddleware(s.config, store, s.log)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.RemoteAddr = "192.168.1.4:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	s.Require().NotEmpty(rec.Header().Get(middleware.RateLimitLimitHeader), "RateLimit-Limit header should be set")
	s.Require().NotEmpty(rec.Header().Get(middleware.RateLimitRemainingHeader), "RateLimit-Remaining header should be set")
	s.Require().NotEmpty(rec.Header().Get(middleware.RateLimitResetHeader), "RateLimit-Reset header should be set")
}

func (s *RateLimitingTestSuite) TestRetryAfterHeaderOnRateLimited() {
	s.T().Parallel()

	store, err := memstore.NewCtx(100)
	s.Require().NoError(err)

	cfg := s.config
	cfg.RequestsPerSecond = 1
	cfg.BurstSize = 0

	handler := middleware.ThrottledRateLimitingMiddleware(cfg, store, s.log)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// First request succeeds
	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.RemoteAddr = "192.168.1.5:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Second request is rate limited
	req = httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.RemoteAddr = "192.168.1.5:12345"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	s.Require().Equal(http.StatusTooManyRequests, rec.Code)
	s.Require().NotEmpty(rec.Header().Get(middleware.RetryAfterHeader), "Retry-After header should be set on 429")
}

func (s *RateLimitingTestSuite) TestIPBasedKeyGeneration() {
	s.T().Parallel()

	store, err := memstore.NewCtx(100)
	s.Require().NoError(err)

	cfg := s.config
	cfg.RequestsPerSecond = 1
	cfg.BurstSize = 0
	cfg.EnableUserLimiting = false

	handler := middleware.ThrottledRateLimitingMiddleware(cfg, store, s.log)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// First IP - should succeed
	req1 := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req1.RemoteAddr = "192.168.1.6:12345"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	s.Require().Equal(http.StatusOK, rec1.Code)

	// Same IP - should be rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req2.RemoteAddr = "192.168.1.6:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	s.Require().Equal(http.StatusTooManyRequests, rec2.Code)

	// Different IP - should succeed
	req3 := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req3.RemoteAddr = "192.168.1.7:12345"
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	s.Require().Equal(http.StatusOK, rec3.Code)
}

func (s *RateLimitingTestSuite) TestUserBasedKeyGeneration() {
	s.T().Parallel()

	store, err := memstore.NewCtx(100)
	s.Require().NoError(err)

	cfg := s.config
	cfg.RequestsPerSecond = 1
	cfg.BurstSize = 0
	cfg.EnableIPLimiting = false
	cfg.EnableUserLimiting = true

	handler := middleware.ThrottledRateLimitingMiddleware(cfg, store, s.log)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	claims := &model.PasetoClaims{
		Subject: "user-123",
	}
	ctx := context.WithValue(context.Background(), middleware.ClaimsKey, claims)

	// First request with user - should succeed
	req1 := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req1 = req1.WithContext(ctx)
	req1.RemoteAddr = "192.168.1.8:12345"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	s.Require().Equal(http.StatusOK, rec1.Code)

	// Same user - should be rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req2 = req2.WithContext(ctx)
	req2.RemoteAddr = "192.168.1.8:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	s.Require().Equal(http.StatusTooManyRequests, rec2.Code)

	// Different user - should succeed
	claims2 := &model.PasetoClaims{
		Subject: "user-456",
	}
	ctx2 := context.WithValue(context.Background(), middleware.ClaimsKey, claims2)
	req3 := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req3 = req3.WithContext(ctx2)
	req3.RemoteAddr = "192.168.1.8:12345"
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	s.Require().Equal(http.StatusOK, rec3.Code)
}

func (s *RateLimitingTestSuite) TestCombinedIPAndUserKeyGeneration() {
	s.T().Parallel()

	store, err := memstore.NewCtx(100)
	s.Require().NoError(err)

	cfg := s.config
	cfg.RequestsPerSecond = 1
	cfg.BurstSize = 0
	cfg.EnableIPLimiting = true
	cfg.EnableUserLimiting = true

	handler := middleware.ThrottledRateLimitingMiddleware(cfg, store, s.log)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	claims := &model.PasetoClaims{
		Subject: "user-789",
	}
	ctx := context.WithValue(context.Background(), middleware.ClaimsKey, claims)

	// Same user + same IP - should be rate limited after first request
	req1 := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req1 = req1.WithContext(ctx)
	req1.RemoteAddr = "192.168.1.9:12345"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	s.Require().Equal(http.StatusOK, rec1.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req2 = req2.WithContext(ctx)
	req2.RemoteAddr = "192.168.1.9:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	s.Require().Equal(http.StatusTooManyRequests, rec2.Code)

	// Same user + different IP - should succeed (different key)
	req3 := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req3 = req3.WithContext(ctx)
	req3.RemoteAddr = "192.168.1.10:12345"
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	s.Require().Equal(http.StatusOK, rec3.Code)
}

func (s *RateLimitingTestSuite) TestRateLimitHeaderValues() {
	s.T().Parallel()

	store, err := memstore.NewCtx(100)
	s.Require().NoError(err)

	cfg := s.config
	cfg.RequestsPerSecond = 10
	cfg.BurstSize = 5

	handler := middleware.ThrottledRateLimitingMiddleware(cfg, store, s.log)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.RemoteAddr = "192.168.1.101:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// RateLimit-Limit should be burst size + 1 (for GCRA)
	limitHeader := rec.Header().Get(middleware.RateLimitLimitHeader)
	limit, err := strconv.Atoi(limitHeader)
	s.Require().NoError(err)
	s.Require().Equal(cfg.BurstSize+1, uint(limit))

	// RateLimit-Remaining should be less than limit after one request
	remainingHeader := rec.Header().Get(middleware.RateLimitRemainingHeader)
	remaining, err := strconv.Atoi(remainingHeader)
	s.Require().NoError(err)
	s.Require().Less(remaining, limit)

	// RateLimit-Reset should be a valid Unix timestamp (within reasonable range)
	resetHeader := rec.Header().Get(middleware.RateLimitResetHeader)
	resetTime, err := strconv.ParseInt(resetHeader, 10, 64)
	s.Require().NoError(err)
	// Reset time should be within 10 seconds of now (allows for timing variations)
	s.Require().GreaterOrEqual(resetTime, time.Now().Unix()-1)
	s.Require().LessOrEqual(resetTime, time.Now().Unix()+10)
}

func (s *RateLimitingTestSuite) TestGlobalKeyWhenBothDisabled() {
	s.T().Parallel()

	store, err := memstore.NewCtx(100)
	s.Require().NoError(err)

	cfg := s.config
	cfg.RequestsPerSecond = 1
	cfg.BurstSize = 0
	cfg.EnableIPLimiting = false
	cfg.EnableUserLimiting = false

	handler := middleware.ThrottledRateLimitingMiddleware(cfg, store, s.log)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// First request from any IP should succeed
	req1 := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req1.RemoteAddr = "192.168.1.12:12345"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	s.Require().Equal(http.StatusOK, rec1.Code)

	// Second request from different IP should be rate limited (global key)
	req2 := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req2.RemoteAddr = "192.168.1.13:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	s.Require().Equal(http.StatusTooManyRequests, rec2.Code)
}

func (s *RateLimitingTestSuite) TestGracefulDegradationOnStoreError() {
	s.T().Parallel()

	// Use a mock store that returns errors
	mockStore := &errorStore{}

	cfg := s.config
	cfg.GracefulDegraded = true

	handler := middleware.ThrottledRateLimitingMiddleware(cfg, mockStore, s.log)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.RemoteAddr = "192.168.1.14:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should allow request through on store error when graceful degradation is enabled
	s.Require().Equal(http.StatusOK, rec.Code)
}

func (s *RateLimitingTestSuite) TestHandlerCalledOnlyOnce() {
	s.T().Parallel()

	store, err := memstore.NewCtx(100)
	s.Require().NoError(err)

	callCount := 0
	handler := middleware.ThrottledRateLimitingMiddleware(s.config, store, s.log)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.RemoteAddr = "192.168.1.15:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	s.Require().Equal(1, callCount, "handler should be called exactly once")
}

func (s *RateLimitingTestSuite) TestSkipPathsWithPrefix() {
	s.T().Parallel()

	cfg := s.config
	cfg.RequestsPerSecond = 1
	cfg.BurstSize = 0
	cfg.SkipPaths = []string{"/v1/health"}

	cases := []struct {
		name       string
		path       string
		ip         string
		shouldSkip bool
	}{
		{
			name:       "exact match",
			path:       "/v1/health",
			ip:         "192.168.2.1:12345",
			shouldSkip: true,
		},
		{
			name:       "subpath match",
			path:       "/v1/health/detailed",
			ip:         "192.168.2.2:12345",
			shouldSkip: true,
		},
		{
			name:       "no match",
			path:       "/v1/devices",
			ip:         "192.168.2.3:12345",
			shouldSkip: false,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			// Create fresh store for each subtest
			store, err := memstore.NewCtx(100)
			s.Require().NoError(err)

			handler := middleware.ThrottledRateLimitingMiddleware(cfg, store, s.log)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}),
			)

			// Make first request
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.RemoteAddr = tc.ip
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			s.Require().Equal(http.StatusOK, rec.Code)

			// Make second request
			req = httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.RemoteAddr = tc.ip
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if tc.shouldSkip {
				s.Require().Equal(http.StatusOK, rec.Code, "skip path should not be rate limited")
			} else {
				s.Require().Equal(http.StatusTooManyRequests, rec.Code, "non-skip path should be rate limited")
			}
		})
	}
}

// errorStore is a mock store that always returns errors.
type errorStore struct{}

func (s *errorStore) GetWithTime(ctx context.Context, key string) (int64, time.Time, error) {
	return 0, time.Time{}, errors.New("store unavailable")
}

func (s *errorStore) SetIfNotExistsWithTTL(ctx context.Context, key string, value int64, ttl time.Duration) (bool, error) {
	return false, errors.New("store unavailable")
}

func (s *errorStore) CompareAndSwapWithTTL(ctx context.Context, key string, old, new int64, ttl time.Duration) (bool, error) {
	return false, errors.New("store unavailable")
}
