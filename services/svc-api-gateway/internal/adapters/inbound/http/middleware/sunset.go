package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
)

// Sunset adds RFC 8594 compliant deprecation headers to responses.
// When enabled, it adds:
//   - Deprecation: true (indicates the API is deprecated)
//   - Sunset: <date> (indicates when the API will be removed)
//   - Link: <url>; rel="successor-version" (points to the new version)
func Sunset(cfg config.Deprecation) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.Enabled {
				w.Header().Set("Deprecation", "true")

				if cfg.SunsetDate != "" {
					sunsetTime, err := time.Parse(time.RFC3339, cfg.SunsetDate)
					if err == nil {
						w.Header().Set("Sunset", sunsetTime.UTC().Format(http.TimeFormat))
					}
				}

				if cfg.SuccessorPath != "" {
					w.Header().Set("Link", fmt.Sprintf("<%s>; rel=\"successor-version\"", cfg.SuccessorPath))
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
