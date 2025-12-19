package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/inbound/http/handlers"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/suite"
	otelNoop "go.opentelemetry.io/otel/trace/noop"
)

type mockDeviceService struct {
	createDeviceFn func(ctx context.Context, name, brand string, state model.State) (*model.Device, error)
	getDeviceFn    func(ctx context.Context, id model.DeviceID) (*model.Device, error)
	listDevicesFn  func(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error)
	updateDeviceFn func(ctx context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error)
	patchDeviceFn  func(ctx context.Context, id model.DeviceID, updates map[string]any) (*model.Device, error)
	deleteDeviceFn func(ctx context.Context, id model.DeviceID) error
}

func (m *mockDeviceService) CreateDevice(ctx context.Context, name, brand string, state model.State) (*model.Device, error) {
	if m.createDeviceFn != nil {
		return m.createDeviceFn(ctx, name, brand, state)
	}

	return model.NewDevice(name, brand, state), nil
}

func (m *mockDeviceService) GetDevice(ctx context.Context, id model.DeviceID) (*model.Device, error) {
	if m.getDeviceFn != nil {
		return m.getDeviceFn(ctx, id)
	}

	return &model.Device{
		ID:        id,
		Name:      "Test Device",
		Brand:     "Test Brand",
		State:     model.StateAvailable,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func (m *mockDeviceService) ListDevices(ctx context.Context, filter model.DeviceFilter) (*model.DeviceList, error) {
	if m.listDevicesFn != nil {
		return m.listDevicesFn(ctx, filter)
	}

	return &model.DeviceList{
		Devices: []*model.Device{},
		Pagination: model.Pagination{
			Page:        filter.Page,
			Size:        filter.Size,
			TotalItems:  0,
			TotalPages:  1,
			HasNext:     false,
			HasPrevious: false,
		},
		Filters: filter,
	}, nil
}

func (m *mockDeviceService) UpdateDevice(ctx context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error) {
	if m.updateDeviceFn != nil {
		return m.updateDeviceFn(ctx, id, name, brand, state)
	}

	return &model.Device{
		ID:        id,
		Name:      name,
		Brand:     brand,
		State:     state,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func (m *mockDeviceService) PatchDevice(ctx context.Context, id model.DeviceID, updates map[string]any) (*model.Device, error) {
	if m.patchDeviceFn != nil {
		return m.patchDeviceFn(ctx, id, updates)
	}

	return &model.Device{
		ID:        id,
		Name:      "Patched",
		Brand:     "Patched Brand",
		State:     model.StateAvailable,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func (m *mockDeviceService) DeleteDevice(ctx context.Context, id model.DeviceID) error {
	if m.deleteDeviceFn != nil {
		return m.deleteDeviceFn(ctx, id)
	}

	return nil
}

func (m *mockDeviceService) Liveness(_ context.Context) (*model.LivenessReport, error) {
	return &model.LivenessReport{
		Status:    model.HealthStatusOK,
		Timestamp: time.Now().UTC(),
		Version:   "1.0.0",
	}, nil
}

func (m *mockDeviceService) Readiness(_ context.Context) (*model.ReadinessReport, error) {
	return &model.ReadinessReport{
		Status:    model.HealthStatusOK,
		Timestamp: time.Now().UTC(),
		Version:   "1.0.0",
		Checks: map[string]model.DependencyCheck{
			"storage": {
				Status:      model.DependencyStatusUp,
				LatencyMs:   0,
				Message:     "ok",
				LastChecked: time.Now().UTC(),
			},
		},
	}, nil
}

func (m *mockDeviceService) Health(_ context.Context) (*model.HealthReport, error) {
	return &model.HealthReport{
		Status:    model.HealthStatusOK,
		Timestamp: time.Now().UTC(),
		Version: model.VersionInfo{
			API:   "1.0.0",
			Build: "development",
			Go:    "1.23",
		},
		Checks: map[string]model.DependencyCheck{
			"storage": {
				Status:      model.DependencyStatusUp,
				LatencyMs:   0,
				Message:     "ok",
				LastChecked: time.Now().UTC(),
			},
		},
		System: model.SystemInfo{
			Goroutines: 1,
			CPUCores:   1,
		},
	}, nil
}

func newTestApp(svc *mockDeviceService) *usecases.WebApplication {
	return usecases.NewWebApplication(svc, svc, logger.NewTestLogger(), otelNoop.NewTracerProvider(), noop.NewMetricsClient())
}

type DeviceHandlerTestSuite struct {
	suite.Suite
}

func TestDeviceHandlerTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(DeviceHandlerTestSuite))
}

func (s *DeviceHandlerTestSuite) TestListDevices_Success() {
	s.T().Parallel()

	svc := &mockDeviceService{}
	app := newTestApp(svc)
	handler := handlers.NewDeviceHandler(app)

	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	rec := httptest.NewRecorder()

	handler.ListDevices(rec, req, handlers.ListDevicesParams{})

	s.Require().Equal(http.StatusOK, rec.Code)

	var response handlers.DeviceListResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	s.Require().NoError(err)
	s.Require().NotNil(response.Data)
}

func (s *DeviceHandlerTestSuite) TestCreateDevice_Success() {
	s.T().Parallel()

	svc := &mockDeviceService{}
	app := newTestApp(svc)
	handler := handlers.NewDeviceHandler(app)

	body := map[string]any{
		"name":  "iPhone 15",
		"brand": "Apple",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/devices", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.CreateDevice(rec, req, handlers.CreateDeviceParams{})

	s.Require().Equal(http.StatusCreated, rec.Code)
	s.Require().NotEmpty(rec.Header().Get("Location"))
}

func (s *DeviceHandlerTestSuite) TestCreateDevice_InvalidJSON() {
	s.T().Parallel()

	svc := &mockDeviceService{}
	app := newTestApp(svc)
	handler := handlers.NewDeviceHandler(app)

	req := httptest.NewRequest(http.MethodPost, "/v1/devices", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.CreateDevice(rec, req, handlers.CreateDeviceParams{})

	s.Require().Equal(http.StatusBadRequest, rec.Code)
}

func (s *DeviceHandlerTestSuite) TestGetDevice_Success() {
	s.T().Parallel()

	id := model.NewDeviceID()
	svc := &mockDeviceService{
		getDeviceFn: func(_ context.Context, _ model.DeviceID) (*model.Device, error) {
			return &model.Device{
				ID:        id,
				Name:      "Test Device",
				Brand:     "Test Brand",
				State:     model.StateAvailable,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}, nil
		},
	}
	app := newTestApp(svc)
	handler := handlers.NewDeviceHandler(app)

	req := httptest.NewRequest(http.MethodGet, "/v1/devices/"+id.String(), nil)
	rec := httptest.NewRecorder()

	handler.GetDevice(rec, req, openapi_types.UUID(id.UUID), handlers.GetDeviceParams{})

	s.Require().Equal(http.StatusOK, rec.Code)

	var response handlers.DeviceResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	s.Require().NoError(err)
	s.Require().Equal(id.UUID, uuid.UUID(response.Data.Id))
}

func (s *DeviceHandlerTestSuite) TestGetDevice_NotFound() {
	s.T().Parallel()

	svc := &mockDeviceService{
		getDeviceFn: func(_ context.Context, _ model.DeviceID) (*model.Device, error) {
			return nil, model.ErrDeviceNotFound
		},
	}
	app := newTestApp(svc)
	handler := handlers.NewDeviceHandler(app)

	id := model.NewDeviceID()
	req := httptest.NewRequest(http.MethodGet, "/v1/devices/"+id.String(), nil)
	rec := httptest.NewRecorder()

	handler.GetDevice(rec, req, openapi_types.UUID(id.UUID), handlers.GetDeviceParams{})

	s.Require().Equal(http.StatusNotFound, rec.Code)
}

func (s *DeviceHandlerTestSuite) TestDeleteDevice_Success() {
	s.T().Parallel()

	svc := &mockDeviceService{}
	app := newTestApp(svc)
	handler := handlers.NewDeviceHandler(app)

	id := model.NewDeviceID()
	req := httptest.NewRequest(http.MethodDelete, "/v1/devices/"+id.String(), nil)
	rec := httptest.NewRecorder()

	handler.DeleteDevice(rec, req, openapi_types.UUID(id.UUID), handlers.DeleteDeviceParams{})

	s.Require().Equal(http.StatusNoContent, rec.Code)
}

func (s *DeviceHandlerTestSuite) TestDeleteDevice_NotFound() {
	s.T().Parallel()

	svc := &mockDeviceService{
		deleteDeviceFn: func(_ context.Context, _ model.DeviceID) error {
			return model.ErrDeviceNotFound
		},
	}
	app := newTestApp(svc)
	handler := handlers.NewDeviceHandler(app)

	id := model.NewDeviceID()
	req := httptest.NewRequest(http.MethodDelete, "/v1/devices/"+id.String(), nil)
	rec := httptest.NewRecorder()

	handler.DeleteDevice(rec, req, openapi_types.UUID(id.UUID), handlers.DeleteDeviceParams{})

	s.Require().Equal(http.StatusNotFound, rec.Code)
}

func (s *DeviceHandlerTestSuite) TestDeleteDevice_InUseConflict() {
	s.T().Parallel()

	svc := &mockDeviceService{
		deleteDeviceFn: func(_ context.Context, _ model.DeviceID) error {
			return model.ErrCannotDeleteInUseDevice
		},
	}
	app := newTestApp(svc)
	handler := handlers.NewDeviceHandler(app)

	id := model.NewDeviceID()
	req := httptest.NewRequest(http.MethodDelete, "/v1/devices/"+id.String(), nil)
	rec := httptest.NewRecorder()

	handler.DeleteDevice(rec, req, openapi_types.UUID(id.UUID), handlers.DeleteDeviceParams{})

	s.Require().Equal(http.StatusConflict, rec.Code)
}

func (s *DeviceHandlerTestSuite) TestLivenessCheck_Success() {
	s.T().Parallel()

	svc := &mockDeviceService{}
	app := newTestApp(svc)
	handler := handlers.NewDeviceHandler(app)

	req := httptest.NewRequest(http.MethodGet, "/v1/liveness", nil)
	rec := httptest.NewRecorder()

	handler.LivenessCheck(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)

	var response handlers.LivenessResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	s.Require().NoError(err)
	s.Require().Equal(handlers.LivenessResponseStatusOk, response.Status)
}

func (s *DeviceHandlerTestSuite) TestOptionsDevices() {
	s.T().Parallel()

	svc := &mockDeviceService{}
	app := newTestApp(svc)
	handler := handlers.NewDeviceHandler(app)

	req := httptest.NewRequest(http.MethodOptions, "/v1/devices", nil)
	rec := httptest.NewRecorder()

	handler.OptionsDevices(rec, req)

	s.Require().Equal(http.StatusNoContent, rec.Code)
	s.Require().Contains(rec.Header().Get("Allow"), "GET")
	s.Require().Contains(rec.Header().Get("Allow"), "POST")
}

func (s *DeviceHandlerTestSuite) TestOptionsDevice() {
	s.T().Parallel()

	svc := &mockDeviceService{}
	app := newTestApp(svc)
	handler := handlers.NewDeviceHandler(app)

	id := model.NewDeviceID()
	req := httptest.NewRequest(http.MethodOptions, "/v1/devices/"+id.String(), nil)
	rec := httptest.NewRecorder()

	handler.OptionsDevice(rec, req, openapi_types.UUID(id.UUID))

	s.Require().Equal(http.StatusNoContent, rec.Code)
	s.Require().Contains(rec.Header().Get("Allow"), "GET")
	s.Require().Contains(rec.Header().Get("Allow"), "DELETE")
}
