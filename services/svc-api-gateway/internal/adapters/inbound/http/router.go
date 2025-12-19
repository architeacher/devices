package http

import (
	"net/http"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/handlers"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/middleware"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

const (
	baseURL = "/v1"
)

type RouterConfig struct {
	App           *usecases.WebApplication
	Logger        logger.Logger
	MetricsClient metrics.Client
	Config        *config.ServiceConfig
}

func NewRouter(cfg RouterConfig) http.Handler {
	router := chi.NewRouter()

	// Core middlewares - always applied
	router.Use(chimiddleware.RequestID)
	router.Use(chimiddleware.RealIP)
	router.Use(middleware.Recovery(cfg.Logger))
	router.Use(chimiddleware.Timeout(cfg.Config.HTTPServer.WriteTimeout))
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.CORS([]string{"*"}))

	// Tracing middleware
	if cfg.Config.Telemetry.Traces.Enabled {
		router.Use(middleware.Tracer())
		cfg.Logger.Info().Msg("distributed tracing enabled")
	}

	// OpenAPI request validation with authentication
	swagger, err := handlers.GetSwagger()
	if err != nil {
		cfg.Logger.Fatal().Err(err).Msg("failed to load swagger spec")
	}

	// Set server to match the base URL for proper path matching
	swagger.Servers = openapi3.Servers{
		&openapi3.Server{URL: baseURL},
	}

	requestValidator := middleware.OapiRequestValidatorWithOptions(
		cfg.Logger,
		swagger,
		&middleware.RequestValidatorOptions{
			Options: openapi3filter.Options{
				MultiError:         false,
				AuthenticationFunc: middleware.NewPasetoAuthenticationFunc(cfg.Config.Auth.Enabled, cfg.Config.Auth.SkipPaths),
			},
			ErrorHandler:          middleware.RequestValidationErrHandler,
			SilenceServersWarning: true,
		},
	)
	router.Use(requestValidator)

	// Metrics middleware
	if cfg.Config.Telemetry.Metrics.Enabled {
		metricsMiddleware := middleware.NewMetricsMiddleware(cfg.MetricsClient)
		router.Use(metricsMiddleware.Middleware)
		cfg.Logger.Info().Msg("HTTP metrics collection enabled")
	}

	// Access logging with health check filtering
	if cfg.Config.Logging.AccessLog.Enabled {
		healthFilter := middleware.NewHealthCheckFilter(cfg.Config.Logging.AccessLog.LogHealthChecks)
		accessLogger := middleware.NewAccessLogger(cfg.Logger)

		router.Use(healthFilter.Middleware)
		router.Use(accessLogger.Middleware)
		cfg.Logger.Info().
			Bool("log_health_checks", cfg.Config.Logging.AccessLog.LogHealthChecks).
			Msg("structured access logging enabled")
	}

	if cfg.Config.Auth.Enabled {
		cfg.Logger.Info().Msg("authentication is enabled")
	}

	handler := handlers.NewDeviceHandler(cfg.App)

	return handlers.HandlerWithOptions(handler, handlers.ChiServerOptions{
		BaseRouter: router,
		BaseURL:    baseURL,
	})
}
