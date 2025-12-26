package middleware

import "net/http"

func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Only process CORS if the Origin header is present (actual cross-origin request)
			if origin == "" {
				next.ServeHTTP(w, r)

				return
			}

			allowed := false
			for _, allowedOrigin := range allowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true

					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Request-Id, Correlation-Id, API-Version, If-Match, If-None-Match, traceparent, tracestate, Idempotency-Key, PASETO-Token")
				w.Header().Set("Access-Control-Expose-Headers", "Request-Id, Correlation-Id, RateLimit-Limit, RateLimit-Remaining, RateLimit-Reset, ETag, Location")
				w.Header().Set("Access-Control-Max-Age", "86400")

				// Handle CORS preflight requests (OPTIONS with valid Origin header)
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)

					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
