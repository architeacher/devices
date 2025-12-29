package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"time"

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

	deviceResponse struct {
		Data deviceData `json:"data"`
	}

	deviceListResponse struct {
		Data       []deviceData   `json:"data"`
		Pagination paginationData `json:"pagination"`
	}

	paginationData struct {
		Page        uint  `json:"page"`
		Size        uint  `json:"size"`
		TotalItems  uint  `json:"totalItems"`
		TotalPages  uint  `json:"totalPages"`
		HasNext     *bool `json:"hasNext,omitempty"`
		HasPrevious *bool `json:"hasPrevious,omitempty"`
	}

	DeviceHandler struct {
		app       *usecases.WebApplication
		startTime time.Time
	}
)

func NewDeviceHandler(app *usecases.WebApplication) *DeviceHandler {
	return &DeviceHandler{
		app:       app,
		startTime: time.Now().UTC(),
	}
}

func (h *DeviceHandler) ListDevices(w http.ResponseWriter, r *http.Request, params ListDevicesParams) {
	filter := model.DefaultDeviceFilter()

	if params.Page != nil {
		filter.Page = uint(*params.Page)
	}
	if params.Size != nil {
		filter.Size = uint(*params.Size)
	}
	if params.Brand != nil && len(*params.Brand) > 0 {
		filter.Brands = *params.Brand
	}
	if params.State != nil && len(*params.State) > 0 {
		states := make([]model.State, 0, len(*params.State))
		for _, s := range *params.State {
			states = append(states, model.State(s))
		}
		filter.States = states
	}
	if params.Sort != nil && len(*params.Sort) > 0 {
		filter.Sort = *params.Sort
	}

	result, err := h.app.Queries.ListDevices.Execute(r.Context(), queries.ListDevicesQuery{Filter: filter})
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, codeInternalError, err.Error())

		return
	}

	writeJSONResponse(w, http.StatusOK, toDeviceListResponse(result))
}

func (h *DeviceHandler) HeadDevices(w http.ResponseWriter, r *http.Request, params HeadDevicesParams) {
	filter := model.DefaultDeviceFilter()

	if params.Page != nil {
		filter.Page = uint(*params.Page)
	}
	if params.Size != nil {
		filter.Size = uint(*params.Size)
	}

	result, err := h.app.Queries.ListDevices.Execute(r.Context(), queries.ListDevicesQuery{Filter: filter})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	w.Header().Set("Total-Count", fmt.Sprintf("%d", result.Pagination.TotalItems))
	w.WriteHeader(http.StatusOK)
}

func (h *DeviceHandler) OptionsDevices(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Allow", "GET, POST, HEAD, OPTIONS")
	w.WriteHeader(http.StatusNoContent)
}

func (h *DeviceHandler) CreateDevice(w http.ResponseWriter, r *http.Request, _ CreateDeviceParams) {
	var req CreateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, codeInvalidJSON, msgInvalidRequestBody)

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
		writeErrorResponse(w, http.StatusInternalServerError, codeInternalError, err.Error())

		return
	}

	w.Header().Set("Location", fmt.Sprintf("/v1/devices/%s", device.ID.String()))
	writeJSONResponse(w, http.StatusCreated, toDeviceResponse(device))
}

func (h *DeviceHandler) GetDevice(w http.ResponseWriter, r *http.Request, deviceId openapi_types.UUID, _ GetDeviceParams) {
	id, err := model.ParseDeviceID(deviceId.String())
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, codeInvalidID, msgInvalidDeviceID)

		return
	}

	device, err := h.app.Queries.GetDevice.Execute(r.Context(), queries.GetDeviceQuery{ID: id})
	if err != nil {
		if errors.Is(err, model.ErrDeviceNotFound) {
			writeErrorResponse(w, http.StatusNotFound, codeNotFound, msgDeviceNotFound)

			return
		}

		writeErrorResponse(w, http.StatusInternalServerError, codeInternalError, err.Error())

		return
	}

	writeJSONResponse(w, http.StatusOK, toDeviceResponse(device))
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

func (h *DeviceHandler) OptionsDevice(w http.ResponseWriter, _ *http.Request, _ openapi_types.UUID) {
	w.Header().Set("Allow", "GET, PUT, PATCH, DELETE, HEAD, OPTIONS")
	w.WriteHeader(http.StatusNoContent)
}

func (h *DeviceHandler) UpdateDevice(w http.ResponseWriter, r *http.Request, deviceId openapi_types.UUID, _ UpdateDeviceParams) {
	id, err := model.ParseDeviceID(deviceId.String())
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, codeInvalidID, msgInvalidDeviceID)

		return
	}

	var req UpdateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, codeInvalidJSON, msgInvalidRequestBody)

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

	writeJSONResponse(w, http.StatusOK, toDeviceResponse(device))
}

func (h *DeviceHandler) PatchDevice(w http.ResponseWriter, r *http.Request, deviceId openapi_types.UUID, _ PatchDeviceParams) {
	id, err := model.ParseDeviceID(deviceId.String())
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, codeInvalidID, msgInvalidDeviceID)

		return
	}

	var req PatchDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, codeInvalidJSON, msgInvalidRequestBody)

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

	writeJSONResponse(w, http.StatusOK, toDeviceResponse(device))
}

func (h *DeviceHandler) DeleteDevice(w http.ResponseWriter, r *http.Request, deviceId openapi_types.UUID, _ DeleteDeviceParams) {
	id, err := model.ParseDeviceID(deviceId.String())
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, codeInvalidID, msgInvalidDeviceID)

		return
	}

	cmd := commands.DeleteDeviceCommand{ID: id}

	_, err = h.app.Commands.DeleteDevice.Handle(r.Context(), cmd)
	if err != nil {
		if errors.Is(err, model.ErrDeviceNotFound) {
			writeErrorResponse(w, http.StatusNotFound, codeNotFound, msgDeviceNotFound)

			return
		}
		if errors.Is(err, model.ErrCannotDeleteInUseDevice) {
			writeErrorResponse(w, http.StatusConflict, codeConflict, msgCannotDeleteInUse)

			return
		}

		writeErrorResponse(w, http.StatusInternalServerError, codeInternalError, err.Error())

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *DeviceHandler) LivenessCheck(w http.ResponseWriter, r *http.Request) {
	result, err := h.app.Queries.FetchLiveness.Execute(r.Context(), queries.FetchLivenessQuery{})
	if err != nil {
		writeJSONResponse(w, http.StatusServiceUnavailable, LivenessResponse{
			Status:    LivenessResponseStatusDown,
			Timestamp: time.Now().UTC(),
			Version:   "1.0.0",
		})

		return
	}

	writeJSONResponse(w, http.StatusOK, LivenessResponse{
		Status:    LivenessResponseStatusOk,
		Timestamp: result.Timestamp,
		Version:   result.Version,
	})
}

func (h *DeviceHandler) ReadinessCheck(w http.ResponseWriter, r *http.Request) {
	result, err := h.app.Queries.FetchReadiness.Execute(r.Context(), queries.FetchReadinessQuery{})
	if err != nil {
		writeJSONResponse(w, http.StatusServiceUnavailable, ReadinessResponse{
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

	writeJSONResponse(w, httpStatus, ReadinessResponse{
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

func writeErrorResponse(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set(contentTypeHeader, applicationJSON)
	w.WriteHeader(status)

	response := ErrorResponse{
		Code:      code,
		Message:   message,
		Timestamp: time.Now().UTC(),
	}

	_ = json.NewEncoder(w).Encode(response)
}

func handleDeviceUpdateError(w http.ResponseWriter, err error) {
	if errors.Is(err, model.ErrDeviceNotFound) {
		writeErrorResponse(w, http.StatusNotFound, codeNotFound, msgDeviceNotFound)

		return
	}

	if errors.Is(err, model.ErrCannotUpdateInUseDevice) {
		writeErrorResponse(w, http.StatusConflict, codeConflict, msgCannotUpdateInUse)

		return
	}

	writeErrorResponse(w, http.StatusInternalServerError, codeInternalError, err.Error())
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

func toDeviceResponse(device *model.Device) deviceResponse {
	return deviceResponse{Data: toDeviceData(device)}
}

func toDeviceListResponse(list *model.DeviceList) deviceListResponse {
	data := make([]deviceData, 0, len(list.Devices))
	for index := range list.Devices {
		data = append(data, toDeviceData(list.Devices[index]))
	}

	hasNext := list.Pagination.HasNext
	hasPrevious := list.Pagination.HasPrevious

	return deviceListResponse{
		Data: data,
		Pagination: paginationData{
			Page:        list.Pagination.Page,
			Size:        list.Pagination.Size,
			TotalItems:  list.Pagination.TotalItems,
			TotalPages:  list.Pagination.TotalPages,
			HasNext:     &hasNext,
			HasPrevious: &hasPrevious,
		},
	}
}
