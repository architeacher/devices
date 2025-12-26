package middleware_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/middleware"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/mocks"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/stretchr/testify/suite"
)

type IdempotencyMiddlewareTestSuite struct {
	suite.Suite
	mockCache *mocks.FakeIdempotencyCache
	handler   func(http.Handler) http.Handler
	log       logger.Logger
	cfg       config.Idempotency
}

func TestIdempotencyMiddlewareSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(IdempotencyMiddlewareTestSuite))
}

func (s *IdempotencyMiddlewareTestSuite) SetupTest() {
	s.mockCache = new(mocks.FakeIdempotencyCache)
	s.log = logger.New("debug", "console")
	s.cfg = config.Idempotency{
		Enabled:          true,
		CacheTTL:         24 * time.Hour,
		LockTTL:          30 * time.Second,
		RequiredMethods:  []string{"POST"},
		HeaderName:       "Idempotency-Key",
		ReplayedHeader:   "Idempotent-Replayed",
		GracefulDegraded: true,
	}
	s.handler = middleware.IdempotencyMiddleware(s.mockCache, s.cfg, s.log)
}

func (s *IdempotencyMiddlewareTestSuite) TestMiddleware_SkipsWhenDisabled() {
	cfg := s.cfg
	cfg.Enabled = false
	handler := middleware.IdempotencyMiddleware(s.mockCache, cfg, s.log)

	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/devices", nil)
	req.Header.Set("Idempotency-Key", "550e8400-e29b-41d4-a716-446655440000")
	rec := httptest.NewRecorder()

	handler(next).ServeHTTP(rec, req)

	s.Require().True(handlerCalled)
	s.Require().Zero(s.mockCache.GetCallCount())
}

func (s *IdempotencyMiddlewareTestSuite) TestMiddleware_SkipsNonMutatingMethods() {
	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	req.Header.Set("Idempotency-Key", "550e8400-e29b-41d4-a716-446655440000")
	rec := httptest.NewRecorder()

	s.handler(next).ServeHTTP(rec, req)

	s.Require().True(handlerCalled)
	s.Require().Zero(s.mockCache.GetCallCount())
}

func (s *IdempotencyMiddlewareTestSuite) TestMiddleware_SkipsWithoutIdempotencyKey() {
	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/devices", nil)
	rec := httptest.NewRecorder()

	s.handler(next).ServeHTTP(rec, req)

	s.Require().True(handlerCalled)
	s.Require().Zero(s.mockCache.GetCallCount())
}

func (s *IdempotencyMiddlewareTestSuite) TestMiddleware_ReturnsErrorForInvalidKey() {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Fail("handler should not be called")
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/devices", nil)
	req.Header.Set("Idempotency-Key", "short")
	rec := httptest.NewRecorder()

	s.handler(next).ServeHTTP(rec, req)

	s.Require().Equal(http.StatusBadRequest, rec.Code)

	var errResp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &errResp)
	s.Require().NoError(err)
	s.Require().Equal("INVALID_IDEMPOTENCY_KEY", errResp["code"])
}

func (s *IdempotencyMiddlewareTestSuite) TestMiddleware_ReturnsCachedResponse() {
	cachedResponse := &ports.CachedResponse{
		StatusCode: http.StatusCreated,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"data":{"id":"123"}}`),
		CreatedAt:  time.Now().UTC(),
	}
	s.mockCache.GetReturns(cachedResponse, nil)

	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/devices", nil)
	req.Header.Set("Idempotency-Key", "550e8400-e29b-41d4-a716-446655440000")
	rec := httptest.NewRecorder()

	s.handler(next).ServeHTTP(rec, req)

	s.Require().False(handlerCalled)
	s.Require().Equal(http.StatusCreated, rec.Code)
	s.Require().Equal("application/json", rec.Header().Get("Content-Type"))
	s.Require().Equal("true", rec.Header().Get("Idempotent-Replayed"))
	s.Require().Equal(`{"data":{"id":"123"}}`, rec.Body.String())
}

func (s *IdempotencyMiddlewareTestSuite) TestMiddleware_ExecutesAndCachesOnMiss() {
	s.mockCache.GetReturns(nil, nil)
	s.mockCache.SetLockReturns(true, nil)
	s.mockCache.SetReturns(nil)

	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"id":"new-id"}}`))
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/devices", bytes.NewReader([]byte(`{"name":"test"}`)))
	req.Header.Set("Idempotency-Key", "550e8400-e29b-41d4-a716-446655440000")
	rec := httptest.NewRecorder()

	s.handler(next).ServeHTTP(rec, req)

	s.Require().True(handlerCalled)
	s.Require().Equal(http.StatusCreated, rec.Code)
	s.Require().Equal(1, s.mockCache.SetCallCount())

	_, key, response, ttl := s.mockCache.SetArgsForCall(0)
	s.Require().NotEmpty(key)
	s.Require().Equal(http.StatusCreated, response.StatusCode)
	s.Require().Equal([]byte(`{"data":{"id":"new-id"}}`), response.Body)
	s.Require().Equal(s.cfg.CacheTTL, ttl)
}

func (s *IdempotencyMiddlewareTestSuite) TestMiddleware_ReturnsConflictWhenLocked() {
	s.mockCache.GetReturns(nil, nil)
	s.mockCache.SetLockReturns(false, nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Fail("handler should not be called")
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/devices", nil)
	req.Header.Set("Idempotency-Key", "550e8400-e29b-41d4-a716-446655440000")
	rec := httptest.NewRecorder()

	s.handler(next).ServeHTTP(rec, req)

	s.Require().Equal(http.StatusConflict, rec.Code)

	var errResp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &errResp)
	s.Require().NoError(err)
	s.Require().Equal("REQUEST_IN_PROGRESS", errResp["code"])
}

func (s *IdempotencyMiddlewareTestSuite) TestMiddleware_DoesNotCacheNon2xx() {
	s.mockCache.GetReturns(nil, nil)
	s.mockCache.SetLockReturns(true, nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"validation failed"}`))
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/devices", nil)
	req.Header.Set("Idempotency-Key", "550e8400-e29b-41d4-a716-446655440000")
	rec := httptest.NewRecorder()

	s.handler(next).ServeHTTP(rec, req)

	s.Require().Equal(http.StatusBadRequest, rec.Code)
	s.Require().Zero(s.mockCache.SetCallCount())
}

func (s *IdempotencyMiddlewareTestSuite) TestMiddleware_GracefulDegradationOnCacheError() {
	s.mockCache.GetReturns(nil, io.ErrUnexpectedEOF)

	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/devices", nil)
	req.Header.Set("Idempotency-Key", "550e8400-e29b-41d4-a716-446655440000")
	rec := httptest.NewRecorder()

	s.handler(next).ServeHTTP(rec, req)

	s.Require().True(handlerCalled)
	s.Require().Equal(http.StatusCreated, rec.Code)
}
