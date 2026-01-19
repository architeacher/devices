package http

import (
	"net/http"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/handlers/public"
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

	// Configure HTTP caching for the handler
	cacheConfig := public.HTTPCacheConfig{
		Enabled:              cfg.ServiceConfig.DevicesCache.HTTPCachingEnabled,
		MaxAge:               cfg.ServiceConfig.DevicesCache.MaxAge,
		StaleWhileRevalidate: cfg.ServiceConfig.DevicesCache.StaleWhileRevalidate,
		ListMaxAge:           cfg.ServiceConfig.DevicesCache.ListMaxAge,
		ListStaleRevalidate:  cfg.ServiceConfig.DevicesCache.ListStaleRevalidate,
	}

	handler := public.NewDeviceHandler(cfg.App, public.WithHTTPCacheConfig(cacheConfig))

	// Spin up automatic generated routes.
	return public.HandlerWithOptions(handler, public.ChiServerOptions{
		BaseRouter:       router,
		BaseURL:          baseURL,
		Middlewares:      initMiddlewares(cfg),
		ErrorHandlerFunc: nil,
	})
}

func initMiddlewares(cfg RouterConfig) []public.MiddlewareFunc {
	swagger, err := public.GetSwagger()
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

	middlewares := []public.MiddlewareFunc{
		chimiddleware.RealIP,
		chimiddleware.Timeout(cfg.ServiceConfig.PublicHTTPServer.WriteTimeout),
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

	if cfg.ServiceConfig.Compression.Enabled {
		var compressionMiddleware func(http.Handler) http.Handler

		// Use metrics-enabled middleware when the metrics client is available
		if cfg.MetricsClient != nil && cfg.ServiceConfig.Telemetry.Metrics.Enabled {
			compressionMiddleware = middleware.CompressionMiddlewareWithMetrics(
				cfg.ServiceConfig.Compression,
				cfg.Logger,
				cfg.MetricsClient,
			)
		} else {
			compressionMiddleware = middleware.CompressionMiddleware(
				cfg.ServiceConfig.Compression,
				cfg.Logger,
			)
		}

		middlewares = append(middlewares, compressionMiddleware)

		cfg.Logger.Info().
			Int("level", cfg.ServiceConfig.Compression.Level).
			Int("min_size", cfg.ServiceConfig.Compression.MinSize).
			Bool("metrics_enabled", cfg.MetricsClient != nil && cfg.ServiceConfig.Telemetry.Metrics.Enabled).
			Msg("response compression enabled")
	}

	// ConditionalGET middleware for ETag generation and 304 Not Modified responses.
	// Must be placed after compression to compute ETag on uncompressed content.
	if cfg.ServiceConfig.DevicesCache.HTTPCachingEnabled {
		etagGenerator := middleware.NewETagGenerator()
		conditionalMiddleware := middleware.ConditionalGET(etagGenerator)

		middlewares = append(middlewares, conditionalMiddleware)

		cfg.Logger.Info().
			Uint("max_age", cfg.ServiceConfig.DevicesCache.MaxAge).
			Uint("stale_while_revalidate", cfg.ServiceConfig.DevicesCache.StaleWhileRevalidate).
			Msg("HTTP response caching enabled (ETag + conditional GET)")
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
