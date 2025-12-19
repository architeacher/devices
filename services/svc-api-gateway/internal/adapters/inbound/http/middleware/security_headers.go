package middleware

import "net/http"

func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			w.Header().Set("API-Version", "v1")

			next.ServeHTTP(w, r)
		})
	}
}

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
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-Id, API-Version, If-Match, If-None-Match, traceparent, tracestate, Idempotency-Key")
				w.Header().Set("Access-Control-Expose-Headers", "X-Request-Id, X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset, ETag, Location")
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
