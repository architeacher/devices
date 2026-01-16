package http

import (
	"net/http"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/handlers/admin"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// AdminRouterConfig holds dependencies for the admin router.
type AdminRouterConfig struct {
	App          *usecases.WebApplication
	DevicesCache ports.DevicesCache
	Logger       logger.Logger
}

// NewAdminRouter creates a router for internal admin endpoints.
// These endpoints are intended to run on a separate internal port.
func NewAdminRouter(cfg AdminRouterConfig) http.Handler {
	router := chi.NewRouter()

	router.Use(chimiddleware.RealIP)
	router.Use(chimiddleware.Recoverer)

	if cfg.DevicesCache == nil {
		cfg.Logger.Warn().Msg("admin router: devices cache not available, cache endpoints will return 503")
	}

	adminHandler := admin.NewAdminHandler(cfg.DevicesCache, cfg.App)

	// Use generated routing from oapi-codegen for consistency with OpenAPI spec.
	return admin.HandlerWithOptions(adminHandler, admin.ChiServerOptions{
		BaseRouter: router,
	})
}
