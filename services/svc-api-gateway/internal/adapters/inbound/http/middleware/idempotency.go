package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"slices"
	"time"

	"github.com/architeacher/devices/pkg/idempotency"
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
)

// IdempotencyMiddleware returns the HTTP middleware handler.
func IdempotencyMiddleware(
	cache ports.IdempotencyCache,
	cfg config.Idempotency,
	log logger.Logger,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled || !slices.Contains(cfg.RequiredMethods, r.Method) {
				next.ServeHTTP(w, r)

				return
			}

			idempotencyKey := r.Header.Get(cfg.HeaderName)
			if idempotencyKey == "" {
				next.ServeHTTP(w, r)

				return
			}

			if err := idempotency.Validate(idempotencyKey); err != nil {
				writeError(w, http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", err.Error())

				return
			}

			cacheKey := idempotency.BuildCacheKey(r.Method, r.URL.Path, idempotencyKey)
			ctx := r.Context()

			cached, err := cache.Get(ctx, cacheKey)
			if err != nil {
				handleCacheError(w, r, next, cfg, log, err, "cache get failed")

				return
			}

			if cached != nil {
				writeCachedResponse(w, cfg, cached)

				return
			}

			acquired, err := cache.SetLock(ctx, cacheKey, cfg.LockTTL)
			if err != nil {
				handleCacheError(w, r, next, cfg, log, err, "cache lock failed")

				return
			}

			if !acquired {
				writeError(w, http.StatusConflict, "REQUEST_IN_PROGRESS",
					"a request with this idempotency key is already being processed")

				return
			}

			defer func() {
				if releaseErr := cache.ReleaseLock(ctx, cacheKey); releaseErr != nil {
					log.Warn().Err(releaseErr).
						Str("idempotency_key", idempotencyKey).
						Msg("failed to release lock")
				}
			}()

			ctx = idempotency.WithKey(ctx, idempotencyKey)

			recorder := newResponseRecorder(w)
			next.ServeHTTP(recorder, r.WithContext(ctx))

			if recorder.statusCode >= http.StatusOK && recorder.statusCode < http.StatusMultipleChoices {
				response := &ports.CachedResponse{
					StatusCode: recorder.statusCode,
					Headers:    recorder.capturedHeaders(),
					Body:       recorder.body.Bytes(),
					CreatedAt:  time.Now().UTC(),
				}

				if cacheErr := cache.Set(ctx, cacheKey, response, cfg.CacheTTL); cacheErr != nil {
					log.Warn().Err(cacheErr).
						Str("idempotency_key", idempotencyKey).
						Msg("failed to cache response")
				}
			}
		})
	}
}

func writeCachedResponse(w http.ResponseWriter, cfg config.Idempotency, cached *ports.CachedResponse) {
	for key, value := range cached.Headers {
		w.Header().Set(key, value)
	}

	w.Header().Set(cfg.ReplayedHeader, "true")
	w.WriteHeader(cached.StatusCode)
	_, _ = w.Write(cached.Body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := map[string]any{
		"code":      code,
		"message":   message,
		"timestamp": time.Now().UTC(),
	}

	_ = json.NewEncoder(w).Encode(response)
}

func handleCacheError(
	w http.ResponseWriter,
	r *http.Request,
	next http.Handler,
	cfg config.Idempotency,
	log logger.Logger,
	err error,
	msg string,
) {
	log.Warn().Err(err).Str("msg", msg).Send()

	if cfg.GracefulDegraded {
		next.ServeHTTP(w, r)

		return
	}

	writeError(w, http.StatusServiceUnavailable, "CACHE_UNAVAILABLE",
		"idempotency service temporarily unavailable")
}

// responseRecorder captures the response for caching.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           &bytes.Buffer{},
	}
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)

	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) capturedHeaders() map[string]string {
	headers := make(map[string]string)

	for key, values := range r.ResponseWriter.Header() {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	return headers
}
