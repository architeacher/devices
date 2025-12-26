package middleware

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	appLogger "github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/throttled/throttled/v2"
)

const (
	RateLimitLimitHeader     = "RateLimit-Limit"
	RateLimitRemainingHeader = "RateLimit-Remaining"
	RateLimitResetHeader     = "RateLimit-Reset"
	RetryAfterHeader         = "Retry-After"

	globalRateLimitKey = "global"
)

func ThrottledRateLimitingMiddleware(
	cfg config.ThrottledRateLimiting,
	store throttled.GCRAStoreCtx,
	logger appLogger.Logger,
) func(http.Handler) http.Handler {
	quota := throttled.RateQuota{
		MaxRate:  throttled.PerSec(int(cfg.RequestsPerSecond)),
		MaxBurst: int(cfg.BurstSize),
	}

	rateLimiter, err := throttled.NewGCRARateLimiterCtx(store, quota)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create rate limiter")
	}

	skipPathsSet := make(map[string]struct{}, len(cfg.SkipPaths))
	for _, path := range cfg.SkipPaths {
		skipPathsSet[path] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if shouldSkipRateLimit(r.URL.Path, cfg.SkipPaths, skipPathsSet) {
				next.ServeHTTP(w, r)

				return
			}

			key := generateRateLimitKey(r, cfg)

			limited, result, err := rateLimiter.RateLimitCtx(r.Context(), key, 1)
			if err != nil {
				handleRateLimitError(w, r, next, cfg, logger, err)

				return
			}

			setRateLimitHeaders(w, result)

			if limited {
				writeRateLimitedResponse(w, result.RetryAfter)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func shouldSkipRateLimit(path string, skipPaths []string, skipPathsSet map[string]struct{}) bool {
	if _, exists := skipPathsSet[path]; exists {
		return true
	}

	for _, skipPath := range skipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}

	return false
}

func generateRateLimitKey(r *http.Request, cfg config.ThrottledRateLimiting) string {
	var parts []string

	if cfg.EnableIPLimiting {
		ip := extractIP(r.RemoteAddr)
		parts = append(parts, "ip:"+ip)
	}

	if cfg.EnableUserLimiting {
		if claims := GetClaims(r.Context()); claims != nil && claims.Subject != "" {
			parts = append(parts, "user:"+claims.Subject)
		}
	}

	if len(parts) == 0 {
		return globalRateLimitKey
	}

	return strings.Join(parts, "|")
}

func extractIP(remoteAddr string) string {
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}

	return remoteAddr
}

func setRateLimitHeaders(w http.ResponseWriter, result throttled.RateLimitResult) {
	w.Header().Set(RateLimitLimitHeader, strconv.Itoa(result.Limit))
	w.Header().Set(RateLimitRemainingHeader, strconv.Itoa(result.Remaining))
	w.Header().Set(RateLimitResetHeader, strconv.FormatInt(time.Now().Add(result.ResetAfter).Unix(), 10))
}

func writeRateLimitedResponse(w http.ResponseWriter, retryAfter time.Duration) {
	w.Header().Set(RetryAfterHeader, strconv.Itoa(int(retryAfter.Seconds())))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)

	response := map[string]any{
		"code":      "RATE_LIMIT_EXCEEDED",
		"message":   "too many requests, please try again later",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	_ = json.NewEncoder(w).Encode(response)
}

func handleRateLimitError(
	w http.ResponseWriter,
	r *http.Request,
	next http.Handler,
	cfg config.ThrottledRateLimiting,
	logger appLogger.Logger,
	err error,
) {
	logger.Warn().Err(err).Msg("rate limiter store error")

	if cfg.GracefulDegraded {
		next.ServeHTTP(w, r)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)

	response := map[string]any{
		"code":      "RATE_LIMITER_UNAVAILABLE",
		"message":   "rate limiting service temporarily unavailable",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	_ = json.NewEncoder(w).Encode(response)
}
