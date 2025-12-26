package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/architeacher/devices/pkg/logger"
)

// Recovery returns a middleware that recovers from panics.
func Recovery(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					if rvr == http.ErrAbortHandler {
						// we don't recover http.ErrAbortHandler, so the response
						// to the client is aborted, this should not be logged
						panic(rvr)
					}

					var errMsg string
					switch v := rvr.(type) {
					case string:
						errMsg = v
					case error:
						errMsg = v.Error()
					default:
						errMsg = fmt.Sprintf("%v", v)
					}

					log.Error().
						Str("error", errMsg).
						Str("stack", string(debug.Stack())).
						Str("path", r.URL.Path).
						Str("method", r.Method).
						Msg("panic recovered")

					w.Header().Set("Content-Type", "application/json")

					if r.Header.Get("Connection") != "Upgrade" {
						w.WriteHeader(http.StatusInternalServerError)
					}

					_, _ = w.Write([]byte(`{"code":"INTERNAL_ERROR","message":"internal server error"}`))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
