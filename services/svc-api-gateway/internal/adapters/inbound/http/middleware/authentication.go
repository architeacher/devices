package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
)

const (
	ClaimsKey contextKey = "claims"
)

func Authentication(enabled bool, skipPaths []string) func(http.Handler) http.Handler {
	skipSet := make(map[string]struct{}, len(skipPaths))
	for _, path := range skipPaths {
		skipSet[path] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled {
				next.ServeHTTP(w, r)

				return
			}

			if _, skip := skipSet[r.URL.Path]; skip {
				next.ServeHTTP(w, r)

				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeUnauthorizedResponse(w, "missing authorization header")

				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				writeUnauthorizedResponse(w, "invalid authorization header format")

				return
			}

			token := parts[1]
			if !strings.HasPrefix(token, "v4.") {
				writeUnauthorizedResponse(w, "invalid token format, expected PASETO v4")

				return
			}

			claims := &model.PasetoClaims{
				Subject:    "user-123",
				Issuer:     "api-gateway",
				Audience:   "devices-api",
				Expiration: time.Now().Add(time.Hour),
				IssuedAt:   time.Now(),
				NotBefore:  time.Now(),
				TokenID:    "token-123",
				Roles:      []string{"user"},
			}

			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetClaims(ctx context.Context) *model.PasetoClaims {
	if claims, ok := ctx.Value(ClaimsKey).(*model.PasetoClaims); ok {
		return claims
	}

	return nil
}

func BasicAuth(username, password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok || user != username || pass != password {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				writeUnauthorizedResponse(w, "invalid credentials")

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeUnauthorizedResponse(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	response := map[string]any{
		"code":      "UNAUTHORIZED",
		"message":   message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	_ = json.NewEncoder(w).Encode(response)
}
