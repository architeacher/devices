package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

type RequestValidatorOptions struct {
	Options               openapi3filter.Options
	ErrorHandler          func(w http.ResponseWriter, message string, statusCode int)
	SilenceServersWarning bool
}

func OapiRequestValidatorWithOptions(
	log logger.Logger,
	swagger *openapi3.T,
	options *RequestValidatorOptions,
) func(http.Handler) http.Handler {
	router, err := gorillamux.NewRouter(swagger)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create OpenAPI router")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip validation for OPTIONS requests (CORS preflight)
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)

				return
			}

			statusCode, err := validateRequest(r, router, options)
			if err != nil {
				if options != nil && options.ErrorHandler != nil {
					options.ErrorHandler(w, err.Error(), statusCode)
				} else {
					RequestValidationErrHandler(w, err.Error(), statusCode)
				}

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func validateRequest(
	r *http.Request,
	router routers.Router,
	options *RequestValidatorOptions,
) (int, error) {
	route, pathParams, err := router.FindRoute(r)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("route not found: %w", err)
	}

	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:    r,
		PathParams: pathParams,
		Route:      route,
	}

	if options != nil {
		requestValidationInput.Options = &options.Options
	}

	ctx := r.Context()

	if err := openapi3filter.ValidateRequest(ctx, requestValidationInput); err != nil {
		switch e := err.(type) {
		case *openapi3filter.RequestError:
			return http.StatusBadRequest, fmt.Errorf("request validation failed: %w", e)
		case *openapi3filter.SecurityRequirementsError:
			return http.StatusUnauthorized, fmt.Errorf("security requirements not met: %w", e)
		default:
			return http.StatusInternalServerError, fmt.Errorf("unexpected validation error: %w", err)
		}
	}

	return http.StatusOK, nil
}

func RequestValidationErrHandler(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]any{
		"code":      http.StatusText(statusCode),
		"message":   sanitizeErrorMessage(message),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	_ = json.NewEncoder(w).Encode(response)
}

func sanitizeErrorMessage(message string) string {
	// Remove potentially sensitive path information
	if idx := strings.Index(message, ": "); idx != -1 {
		return message[idx+2:]
	}

	return message
}

func NewPasetoAuthenticationFunc(
	authEnabled bool,
	skipPaths []string,
) openapi3filter.AuthenticationFunc {
	skipSet := make(map[string]struct{}, len(skipPaths))
	for _, path := range skipPaths {
		skipSet[path] = struct{}{}
	}

	return func(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
		if !authEnabled {
			return nil
		}

		// Check if path should skip authentication
		if _, skip := skipSet[input.RequestValidationInput.Request.URL.Path]; skip {
			return nil
		}

		// Get the security scheme name
		securitySchemeName := input.SecuritySchemeName

		switch securitySchemeName {
		case "BearerAuth", "bearerAuth", "PasetoAuth":
			return validateBearerToken(input.RequestValidationInput.Request)
		default:
			return fmt.Errorf("unsupported security scheme: %s", securitySchemeName)
		}
	}
}

func validateBearerToken(r *http.Request) error {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return fmt.Errorf("missing authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return fmt.Errorf("invalid authorization header format")
	}

	token := parts[1]
	if !strings.HasPrefix(token, "v4.") {
		return fmt.Errorf("invalid token format, expected PASETO v4")
	}

	// Token format is valid - actual verification would happen here
	// For now, we accept valid PASETO v4 format tokens

	return nil
}
