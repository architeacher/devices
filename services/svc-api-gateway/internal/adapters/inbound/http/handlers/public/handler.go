package public

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/handlers/shared"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases/commands"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases/queries"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

const (
	contentTypeHeader = "Content-Type"
	applicationJSON   = "application/json"

	codeNotFound      = "NOT_FOUND"
	codeConflict      = "CONFLICT"
	codeInternalError = "INTERNAL_ERROR"
	codeInvalidID     = "INVALID_ID"
	codeInvalidJSON   = "INVALID_JSON"

	msgDeviceNotFound     = "device not found"
	msgInvalidDeviceID    = "invalid device ID"
	msgInvalidRequestBody = "invalid request body"
	msgCannotUpdateInUse  = "cannot update name or brand of in-use device"
	msgCannotDeleteInUse  = "cannot delete in-use device"
)

type (
	deviceLinks struct {
		Self *string `json:"self,omitempty"`
	}

	deviceData struct {
		Brand     string             `json:"brand"`
		CreatedAt time.Time          `json:"createdAt"`
		Id        openapi_types.UUID `json:"id"`
		Links     *deviceLinks       `json:"links,omitempty"`
		Name      string             `json:"name"`
		State     string             `json:"state"`
		UpdatedAt *time.Time         `json:"updatedAt,omitempty"`
	}

	// HTTPCacheConfig holds HTTP caching configuration for the handler.
	HTTPCacheConfig struct {
		Enabled              bool
		MaxAge               uint
		StaleWhileRevalidate uint
		ListMaxAge           uint
		ListStaleRevalidate  uint
	}

	DeviceHandler struct {
		app       *usecases.WebApplication
		cacheConf HTTPCacheConfig
		startTime time.Time
	}

	// DeviceHandlerOption configures the DeviceHandler.
	DeviceHandlerOption func(*DeviceHandler)
)

func NewDeviceHandler(app *usecases.WebApplication, opts ...DeviceHandlerOption) *DeviceHandler {
	h := &DeviceHandler{
		app:       app,
		startTime: time.Now().UTC(),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// WithHTTPCacheConfig sets the HTTP caching configuration.
func WithHTTPCacheConfig(cfg HTTPCacheConfig) DeviceHandlerOption {
	return func(h *DeviceHandler) {
		h.cacheConf = cfg
	}
}

// setCacheControlHeaders sets Cache-Control and Vary headers for cacheable responses.
func (h *DeviceHandler) setCacheControlHeaders(w http.ResponseWriter, isList bool) {
	if !h.cacheConf.Enabled {
		return
	}

	maxAge := h.cacheConf.MaxAge
	staleRevalidate := h.cacheConf.StaleWhileRevalidate

	if isList {
		maxAge = h.cacheConf.ListMaxAge
		staleRevalidate = h.cacheConf.ListStaleRevalidate
	}

	cacheControl := fmt.Sprintf("private, max-age=%d", maxAge)
	if staleRevalidate > 0 {
		cacheControl = fmt.Sprintf("%s, stale-while-revalidate=%d", cacheControl, staleRevalidate)
	}

	w.Header().Set(shared.HeaderCacheControl, cacheControl)
	w.Header().Set(shared.HeaderVary, fmt.Sprintf("%s, %s, Accept-Encoding", shared.HeaderAccept, shared.HeaderAuthorization))
}

// setCacheObservabilityHeaders sets Cache-Status and Cache-Key headers for debugging.
func (h *DeviceHandler) setCacheObservabilityHeaders(w http.ResponseWriter, r *http.Request, cacheKey string) {
	if !h.cacheConf.Enabled {
		return
	}

	// Determine cache status based on request headers
	var status string
	if shared.IsCacheBypassRequested(r) {
		status = "BYPASS"
	} else {
		// Since we don't have visibility into backend cache hits at the HTTP layer,
		// we report MISS. The backend KeyDB caching is separate from HTTP caching.
		status = "MISS"
	}

	w.Header().Set(shared.HeaderCacheStatus, status)

	if cacheKey != "" {
		w.Header().Set(shared.HeaderCacheKey, cacheKey)
	}
}

// DeviceListFilterInput captures common filter parameters for device list operations.
// This struct allows both ListDevices and HeadDevices to share filter construction logic.
type DeviceListFilterInput struct {
	Q      *SearchParam
	Brand  *BrandFilterParam
	State  *StateFilterParam
	Sort   *SortParam
	Page   *PageParam
	Size   *SizeParam
	Cursor *CursorParam
}

// buildDeviceFilter constructs a DeviceFilter from the common list/head parameters.
func buildDeviceFilter(input DeviceListFilterInput) model.DeviceFilter {
	filter := model.DefaultDeviceFilter()

	if input.Q != nil && *input.Q != "" {
		filter.Keyword = *input.Q
	}

	if input.Brand != nil && len(*input.Brand) > 0 {
		filter.Brands = *input.Brand
	}

	if input.State != nil && len(*input.State) > 0 {
		states := make([]model.State, 0, len(*input.State))
		for _, s := range *input.State {
			states = append(states, model.State(s))
		}
		filter.States = states
	}

	if input.Sort != nil && len(*input.Sort) > 0 {
		filter.Sort = *input.Sort
	}

	if input.Page != nil {
		filter.Page = uint(*input.Page)
	}

	if input.Size != nil {
		filter.Size = uint(*input.Size)
	}

	if input.Cursor != nil && *input.Cursor != "" {
		filter.Cursor = *input.Cursor
	}

	return filter
}

func (h *DeviceHandler) ListDevices(w http.ResponseWriter, r *http.Request, params ListDevicesParams) {
	filter := buildDeviceFilter(DeviceListFilterInput{
		Q:      params.Q,
		Brand:  params.Brand,
		State:  params.State,
		Sort:   params.Sort,
		Page:   params.Page,
		Size:   params.Size,
		Cursor: params.Cursor,
	})

	result, err := h.app.Queries.ListDevices.Execute(r.Context(), queries.ListDevicesQuery{Filter: filter})
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternalError, err.Error())

		return
	}

	data, pagination := toDeviceListData(result)
	response := shared.EnvelopedResponse{
		Data:       data,
		Meta:       shared.NewMeta(r),
		Pagination: pagination,
	}

	// Build cache key from filter parameters for list endpoint
	cacheKey := buildListCacheKey(filter)

	h.setCacheControlHeaders(w, true)
	h.setCacheObservabilityHeaders(w, r, cacheKey)
	writeJSONResponse(w, http.StatusOK, response)
}

// buildListCacheKey generates a cache key for list queries based on filter parameters.
func buildListCacheKey(filter model.DeviceFilter) string {
	return fmt.Sprintf("devices:list:page=%d:size=%d:brands=%v:states=%v",
		filter.Page, filter.Size, filter.Brands, filter.States)
}

func (h *DeviceHandler) HeadDevices(w http.ResponseWriter, r *http.Request, params HeadDevicesParams) {
	filter := buildDeviceFilter(DeviceListFilterInput{
		Q:      params.Q,
		Brand:  params.Brand,
		State:  params.State,
		Sort:   params.Sort,
		Page:   params.Page,
		Size:   params.Size,
		Cursor: params.Cursor,
	})

	result, err := h.app.Queries.ListDevices.Execute(r.Context(), queries.ListDevicesQuery{Filter: filter})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	w.Header().Set("Total-Count", fmt.Sprintf("%d", result.Pagination.TotalItems))
	w.WriteHeader(http.StatusOK)
}

func (h *DeviceHandler) OptionsDevices(w http.ResponseWriter, _ *http.Request, _ OptionsDevicesParams) {
	w.Header().Set("Allow", "GET, POST, HEAD, OPTIONS")
	w.WriteHeader(http.StatusNoContent)
}

func (h *DeviceHandler) CreateDevice(w http.ResponseWriter, r *http.Request, _ CreateDeviceParams) {
	var req CreateDevice
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidJSON, msgInvalidRequestBody)

		return
	}

	state := model.StateAvailable
	if req.State != nil {
		state = model.State(*req.State)
	}

	cmd := commands.CreateDeviceCommand{
		Name:  req.Name,
		Brand: req.Brand,
		State: state,
	}

	device, err := h.app.Commands.CreateDevice.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternalError, err.Error())

		return
	}

	w.Header().Set("Location", fmt.Sprintf("/v1/devices/%s", device.ID.String()))

	response := shared.EnvelopedResponse{
		Data: toDeviceData(device),
		Meta: shared.NewMeta(r),
	}

	writeJSONResponse(w, http.StatusCreated, response)
}

func (h *DeviceHandler) GetDevice(w http.ResponseWriter, r *http.Request, deviceId openapi_types.UUID, _ GetDeviceParams) {
	id, err := model.ParseDeviceID(deviceId.String())
	if err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidID, msgInvalidDeviceID)

		return
	}

	device, err := h.app.Queries.GetDevice.Execute(r.Context(), queries.GetDeviceQuery{ID: id})
	if err != nil {
		if errors.Is(err, model.ErrDeviceNotFound) {
			writeError(w, http.StatusNotFound, codeNotFound, msgDeviceNotFound)

			return
		}

		writeError(w, http.StatusInternalServerError, codeInternalError, err.Error())

		return
	}

	response := shared.EnvelopedResponse{
		Data: toDeviceData(device),
		Meta: shared.NewMeta(r),
	}

	// Build cache key for single device endpoint
	cacheKey := fmt.Sprintf("device:v1:%s", device.ID.String())

	h.setCacheControlHeaders(w, false)
	h.setCacheObservabilityHeaders(w, r, cacheKey)
	shared.SetLastModified(w, device.UpdatedAt)
	writeJSONResponse(w, http.StatusOK, response)
}

func (h *DeviceHandler) HeadDevice(w http.ResponseWriter, r *http.Request, deviceId openapi_types.UUID, _ HeadDeviceParams) {
	id, err := model.ParseDeviceID(deviceId.String())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)

		return
	}

	_, err = h.app.Queries.GetDevice.Execute(r.Context(), queries.GetDeviceQuery{ID: id})
	if err != nil {
		if errors.Is(err, model.ErrDeviceNotFound) {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *DeviceHandler) OptionsDevice(w http.ResponseWriter, _ *http.Request, _ DeviceIdParam, _ OptionsDeviceParams) {
	w.Header().Set("Allow", "GET, PUT, PATCH, DELETE, HEAD, OPTIONS")
	w.WriteHeader(http.StatusNoContent)
}

func (h *DeviceHandler) UpdateDevice(w http.ResponseWriter, r *http.Request, deviceId openapi_types.UUID, _ UpdateDeviceParams) {
	id, err := model.ParseDeviceID(deviceId.String())
	if err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidID, msgInvalidDeviceID)

		return
	}

	var req UpdateDevice
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidJSON, msgInvalidRequestBody)

		return
	}

	cmd := commands.UpdateDeviceCommand{
		ID:    id,
		Name:  req.Name,
		Brand: req.Brand,
		State: model.State(req.State),
	}

	device, err := h.app.Commands.UpdateDevice.Handle(r.Context(), cmd)
	if err != nil {
		handleDeviceUpdateError(w, err)

		return
	}

	response := shared.EnvelopedResponse{
		Data: toDeviceData(device),
		Meta: shared.NewMeta(r),
	}

	writeJSONResponse(w, http.StatusOK, response)
}

func (h *DeviceHandler) PatchDevice(w http.ResponseWriter, r *http.Request, deviceId openapi_types.UUID, _ PatchDeviceParams) {
	id, err := model.ParseDeviceID(deviceId.String())
	if err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidID, msgInvalidDeviceID)

		return
	}

	var req PatchDevice
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidJSON, msgInvalidRequestBody)

		return
	}

	updates := make(map[string]any)
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Brand != nil {
		updates["brand"] = *req.Brand
	}
	if req.State != nil {
		updates["state"] = string(*req.State)
	}

	cmd := commands.PatchDeviceCommand{
		ID:      id,
		Updates: updates,
	}

	device, err := h.app.Commands.PatchDevice.Handle(r.Context(), cmd)
	if err != nil {
		handleDeviceUpdateError(w, err)

		return
	}

	response := shared.EnvelopedResponse{
		Data: toDeviceData(device),
		Meta: shared.NewMeta(r),
	}

	writeJSONResponse(w, http.StatusOK, response)
}

func (h *DeviceHandler) DeleteDevice(w http.ResponseWriter, r *http.Request, deviceId openapi_types.UUID, _ DeleteDeviceParams) {
	id, err := model.ParseDeviceID(deviceId.String())
	if err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidID, msgInvalidDeviceID)

		return
	}

	cmd := commands.DeleteDeviceCommand{ID: id}

	_, err = h.app.Commands.DeleteDevice.Handle(r.Context(), cmd)
	if err != nil {
		if errors.Is(err, model.ErrDeviceNotFound) {
			writeError(w, http.StatusNotFound, codeNotFound, msgDeviceNotFound)

			return
		}
		if errors.Is(err, model.ErrCannotDeleteInUseDevice) {
			writeError(w, http.StatusConflict, codeConflict, msgCannotDeleteInUse)

			return
		}

		writeError(w, http.StatusInternalServerError, codeInternalError, err.Error())

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *DeviceHandler) LivenessCheck(w http.ResponseWriter, r *http.Request) {
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

func (h *DeviceHandler) ReadinessCheck(w http.ResponseWriter, r *http.Request) {
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

	if result.Status != "ok" {
		status = Down
		httpStatus = http.StatusServiceUnavailable
	}

	writeJSONResponse(w, httpStatus, Readiness{
		Status:    status,
		Timestamp: result.Timestamp,
	})
}

func (h *DeviceHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	result, err := h.app.Queries.FetchHealthReport.Execute(r.Context(), queries.FetchHealthReportQuery{})
	if err != nil {
		writeJSONResponse(w, http.StatusServiceUnavailable, map[string]any{
			"status":    "down",
			"timestamp": time.Now().UTC(),
		})

		return
	}

	status := "ok"
	httpStatus := http.StatusOK

	if result.Status != "ok" {
		status = "down"
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

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set(contentTypeHeader, applicationJSON)
	w.WriteHeader(status)

	response := Error{
		Code:      code,
		Message:   message,
		Timestamp: time.Now().UTC(),
	}

	_ = json.NewEncoder(w).Encode(response)
}

func handleDeviceUpdateError(w http.ResponseWriter, err error) {
	if errors.Is(err, model.ErrDeviceNotFound) {
		writeError(w, http.StatusNotFound, codeNotFound, msgDeviceNotFound)

		return
	}

	if errors.Is(err, model.ErrCannotUpdateInUseDevice) {
		writeError(w, http.StatusConflict, codeConflict, msgCannotUpdateInUse)

		return
	}

	writeError(w, http.StatusInternalServerError, codeInternalError, err.Error())
}

func toDeviceData(device *model.Device) deviceData {
	selfLink := fmt.Sprintf("/v1/devices/%s", device.ID.String())
	updatedAt := device.UpdatedAt

	return deviceData{
		Id:        device.ID.UUID,
		Name:      device.Name,
		Brand:     device.Brand,
		State:     string(device.State),
		CreatedAt: device.CreatedAt,
		UpdatedAt: &updatedAt,
		Links:     &deviceLinks{Self: &selfLink},
	}
}

func toDeviceListData(list *model.DeviceList) ([]deviceData, *shared.PaginationData) {
	data := make([]deviceData, 0, len(list.Devices))
	for index := range list.Devices {
		data = append(data, toDeviceData(list.Devices[index]))
	}

	hasNext := list.Pagination.HasNext
	hasPrevious := list.Pagination.HasPrevious

	pagination := &shared.PaginationData{
		Page:        list.Pagination.Page,
		Size:        list.Pagination.Size,
		TotalItems:  list.Pagination.TotalItems,
		TotalPages:  list.Pagination.TotalPages,
		HasNext:     &hasNext,
		HasPrevious: &hasPrevious,
	}

	if list.Pagination.NextCursor != "" {
		pagination.NextCursor = &list.Pagination.NextCursor
	}

	if list.Pagination.PreviousCursor != "" {
		pagination.PreviousCursor = &list.Pagination.PreviousCursor
	}

	return data, pagination
}
