package http

import (
	"net/http"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/handlers"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/middleware"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/throttled/throttled/v2"
)

const (
	baseURL = "/v1"
)

type RouterConfig struct {
	ServiceConfig   *config.ServiceConfig
	App             *usecases.WebApplication
	IdempotencyRepo ports.IdempotencyCache
	RateLimitStore  throttled.GCRAStoreCtx
	Logger          logger.Logger
	MetricsClient   metrics.Client
}

func NewRouter(cfg RouterConfig) http.Handler {
	router := chi.NewRouter()

	handler := handlers.NewDeviceHandler(cfg.App)

	// Spin up automatic generated routes.
	return handlers.HandlerWithOptions(handler, handlers.ChiServerOptions{
		BaseRouter:       router,
		BaseURL:          baseURL,
		Middlewares:      initMiddlewares(cfg),
		ErrorHandlerFunc: nil,
	})
}

func initMiddlewares(cfg RouterConfig) []handlers.MiddlewareFunc {
	swagger, err := handlers.GetSwagger()
	if err != nil {
		cfg.Logger.Fatal().Err(err).Msg("failed to load swagger spec")
	}

	// Set server to match the base URL for proper path matching
	swagger.Servers = openapi3.Servers{
		&openapi3.Server{URL: baseURL},
	}

	// OpenAPI request validation with authentication
	requestValidator := middleware.OapiRequestValidatorWithOptions(
		cfg.Logger,
		swagger,
		&middleware.RequestValidatorOptions{
			Options: openapi3filter.Options{
				MultiError:         false,
				AuthenticationFunc: middleware.NewPasetoAuthenticationFunc(cfg.ServiceConfig.Auth.Enabled, cfg.ServiceConfig.Auth.SkipPaths),
			},
			ErrorHandler:          middleware.RequestValidationErrHandler,
			SilenceServersWarning: true,
		},
	)

	middlewares := []handlers.MiddlewareFunc{
		chimiddleware.RealIP,
		chimiddleware.Timeout(cfg.ServiceConfig.HTTPServer.WriteTimeout),
		middleware.RequestTracking(),
		middleware.SecurityHeaders(cfg.ServiceConfig.App.APIVersion),
		middleware.CORS([]string{"*"}),
		middleware.Recovery(cfg.Logger),
		requestValidator,
	}

	if cfg.ServiceConfig.Auth.Enabled {
		cfg.Logger.Info().Msg("authentication is enabled")
	}

	if cfg.ServiceConfig.ThrottledRateLimiting.Enabled && cfg.RateLimitStore != nil {
		rateLimitMiddleware := middleware.ThrottledRateLimitingMiddleware(
			cfg.ServiceConfig.ThrottledRateLimiting,
			cfg.RateLimitStore,
			cfg.Logger,
		)
		middlewares = append(middlewares, rateLimitMiddleware)

		cfg.Logger.Info().Msg("rate limiting enabled")
	}

	if cfg.ServiceConfig.Idempotency.Enabled && cfg.IdempotencyRepo != nil {
		idempotencyMiddleware := middleware.IdempotencyMiddleware(
			cfg.IdempotencyRepo,
			cfg.ServiceConfig.Idempotency,
			cfg.Logger,
		)
		middlewares = append(middlewares, idempotencyMiddleware)

		cfg.Logger.Info().Msg("idempotency middleware enabled")
	}

	if cfg.ServiceConfig.Deprecation.Enabled {
		middlewares = append(middlewares, middleware.Sunset(cfg.ServiceConfig.Deprecation))

		cfg.Logger.Info().
			Str("sunset_date", cfg.ServiceConfig.Deprecation.SunsetDate).
			Str("successor_path", cfg.ServiceConfig.Deprecation.SuccessorPath).
			Msg("API deprecation headers enabled")
	}

	// Access logging with health check filtering.
	if cfg.ServiceConfig.Logging.AccessLog.Enabled {
		accessLogCfg := cfg.ServiceConfig.Logging.AccessLog
		healthFilter := middleware.HealthCheckFilter(accessLogCfg.LogHealthChecks)
		accessLogger := middleware.AccessLogger(cfg.Logger, accessLogCfg.IncludeQueryParams)

		middlewares = append(middlewares, healthFilter, accessLogger)

		cfg.Logger.Info().
			Bool("log_health_checks", accessLogCfg.LogHealthChecks).
			Bool("include_query_params", accessLogCfg.IncludeQueryParams).
			Msg("structured access logging enabled")
	}

	if cfg.ServiceConfig.Telemetry.Metrics.Enabled {
		metricsMiddleware := middleware.MetricsMiddleware(cfg.MetricsClient)
		middlewares = append(middlewares, metricsMiddleware)

		cfg.Logger.Info().Msg("HTTP metrics collection enabled")
	}

	if cfg.ServiceConfig.Telemetry.Traces.Enabled {
		middlewares = append(middlewares, middleware.Tracer())

		cfg.Logger.Info().Msg("distributed tracing enabled")
	}

	return middlewares
}
