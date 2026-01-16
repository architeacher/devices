package admin

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases/queries"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

const (
	contentTypeHeader = "Content-Type"
	applicationJSON   = "application/json"

	statusOK          = "ok"
	statusDown        = "down"
	statusHealthy     = "healthy"
	statusUnhealthy   = "unhealthy"
	statusUnavailable = "unavailable"
)

// AdminHandler provides internal admin endpoints for cache management and system health.
// These endpoints should only be exposed on an internal port, not the public API.
type AdminHandler struct {
	cache     ports.DevicesCache
	app       *usecases.WebApplication
	startTime time.Time
}

// NewAdminHandler creates a new admin handler for cache operations and system health.
func NewAdminHandler(cache ports.DevicesCache, app *usecases.WebApplication) *AdminHandler {
	return &AdminHandler{
		cache:     cache,
		app:       app,
		startTime: time.Now().UTC(),
	}
}

// GetCacheHealth checks if the cache is healthy.
func (h *AdminHandler) GetCacheHealth(w http.ResponseWriter, r *http.Request) {
	if h.cache == nil {
		writeJSONResponse(w, http.StatusServiceUnavailable, map[string]string{
			"status": statusUnavailable,
			"error":  "cache not configured",
		})

		return
	}

	if !h.cache.IsHealthy(r.Context()) {
		writeJSONResponse(w, http.StatusServiceUnavailable, map[string]string{
			"status": statusUnhealthy,
		})

		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]string{
		"status": statusHealthy,
	})
}

// PurgeAllDeviceCaches purges all device-related caches.
func (h *AdminHandler) PurgeAllDeviceCaches(w http.ResponseWriter, r *http.Request) {
	if h.cache == nil {
		writeJSONResponse(w, http.StatusServiceUnavailable, map[string]string{
			"error": "cache not available",
		})

		return
	}

	if err := h.cache.PurgeAll(r.Context()); err != nil {
		writeJSONResponse(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to purge cache: " + err.Error(),
		})

		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]string{
		"status": "all device caches purged",
	})
}

// PurgeDeviceCache purges the cache for a specific device.
func (h *AdminHandler) PurgeDeviceCache(w http.ResponseWriter, r *http.Request, deviceId openapi_types.UUID) {
	if h.cache == nil {
		writeJSONResponse(w, http.StatusServiceUnavailable, map[string]string{
			"error": "cache not available",
		})

		return
	}

	deviceID, err := model.ParseDeviceID(deviceId.String())
	if err != nil {
		writeJSONResponse(w, http.StatusBadRequest, map[string]string{
			"error": "invalid device ID: " + err.Error(),
		})

		return
	}

	if err := h.cache.InvalidateDevice(r.Context(), deviceID); err != nil {
		writeJSONResponse(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to invalidate device cache: " + err.Error(),
		})

		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]string{
		"status": "device cache purged",
		"id":     deviceId.String(),
	})
}

// PurgeDeviceListCaches purges all device list caches.
func (h *AdminHandler) PurgeDeviceListCaches(w http.ResponseWriter, r *http.Request) {
	if h.cache == nil {
		writeJSONResponse(w, http.StatusServiceUnavailable, map[string]string{
			"error": "cache not available",
		})

		return
	}

	if err := h.cache.InvalidateAllLists(r.Context()); err != nil {
		writeJSONResponse(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to purge list caches: " + err.Error(),
		})

		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]string{
		"status": "device list caches purged",
	})
}

// PurgeCacheByPattern purges caches matching a pattern.
func (h *AdminHandler) PurgeCacheByPattern(w http.ResponseWriter, r *http.Request, params PurgeCacheByPatternParams) {
	if h.cache == nil {
		writeJSONResponse(w, http.StatusServiceUnavailable, map[string]string{
			"error": "cache not available",
		})

		return
	}

	count, err := h.cache.PurgeByPattern(r.Context(), params.Pattern)
	if err != nil {
		writeJSONResponse(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to purge by pattern: " + err.Error(),
		})

		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"status":  "cache purged by pattern",
		"pattern": params.Pattern,
		"deleted": count,
	})
}

// LivenessCheck returns simple liveness status.
func (h *AdminHandler) LivenessCheck(w http.ResponseWriter, r *http.Request) {
	result, err := h.app.Queries.FetchLiveness.Execute(r.Context(), queries.FetchLivenessQuery{})
	if err != nil {
		writeJSONResponse(w, http.StatusServiceUnavailable, Liveness{
			Status:    LivenessStatusDown,
			Timestamp: time.Now().UTC(),
			Version:   "1.0.0",
		})

		return
	}

	writeJSONResponse(w, http.StatusOK, Liveness{
		Status:    LivenessStatusOk,
		Timestamp: result.Timestamp,
		Version:   result.Version,
	})
}

// ReadinessCheck returns readiness status with dependency checks.
func (h *AdminHandler) ReadinessCheck(w http.ResponseWriter, r *http.Request) {
	result, err := h.app.Queries.FetchReadiness.Execute(r.Context(), queries.FetchReadinessQuery{})
	if err != nil {
		writeJSONResponse(w, http.StatusServiceUnavailable, Readiness{
			Status:    Down,
			Timestamp: time.Now().UTC(),
		})

		return
	}

	status := Ok
	httpStatus := http.StatusOK

	if result.Status != statusOK {
		status = Down
		httpStatus = http.StatusServiceUnavailable
	}

	writeJSONResponse(w, httpStatus, Readiness{
		Status:    status,
		Timestamp: result.Timestamp,
	})
}

// HealthCheck returns comprehensive health status with system metrics.
func (h *AdminHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	result, err := h.app.Queries.FetchHealthReport.Execute(r.Context(), queries.FetchHealthReportQuery{})
	if err != nil {
		writeJSONResponse(w, http.StatusServiceUnavailable, map[string]any{
			"status":    statusDown,
			"timestamp": time.Now().UTC(),
		})

		return
	}

	status := statusOK
	httpStatus := http.StatusOK

	if result.Status != statusOK {
		status = statusDown
		httpStatus = http.StatusServiceUnavailable
	}

	uptime := time.Since(h.startTime)
	uptimeSeconds := int(uptime.Seconds())
	goVersion := runtime.Version()

	response := map[string]any{
		"status":    status,
		"timestamp": result.Timestamp,
		"version": map[string]any{
			"api":   result.Version.API,
			"build": result.Version.Build,
			"go":    goVersion,
		},
		"uptime": map[string]any{
			"duration":        uptime.String(),
			"durationSeconds": uptimeSeconds,
			"startedAt":       h.startTime,
		},
		"system": map[string]any{
			"cpuCores":   result.System.CPUCores,
			"goroutines": result.System.Goroutines,
		},
	}

	writeJSONResponse(w, httpStatus, response)
}

func writeJSONResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set(contentTypeHeader, applicationJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
