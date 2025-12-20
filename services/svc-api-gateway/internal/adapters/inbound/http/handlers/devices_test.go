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
	"github.com/architeacher/devices/services/svc-api-gateway/internal/mocks"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	otelNoop "go.opentelemetry.io/otel/trace/noop"
)

func newTestApp(deviceSvc *mocks.FakeDevicesService, healthChecker *mocks.FakeHealthChecker) *usecases.WebApplication {
	return usecases.NewWebApplication(deviceSvc, healthChecker, logger.NewTestLogger(), otelNoop.NewTracerProvider(), noop.NewMetricsClient())
}

func newDefaultHealthChecker() *mocks.FakeHealthChecker {
	hc := &mocks.FakeHealthChecker{}
	hc.LivenessReturns(&model.LivenessReport{
		Status:    model.HealthStatusOK,
		Timestamp: time.Now().UTC(),
		Version:   "1.0.0",
	}, nil)
	hc.ReadinessReturns(&model.ReadinessReport{
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
	}, nil)
	hc.HealthReturns(&model.HealthReport{
		Status:    model.HealthStatusOK,
		Timestamp: time.Now().UTC(),
		Version: model.VersionInfo{
			API:   "1.0.0",
			Build: "development",
			Go:    "1.25",
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
	}, nil)

	return hc
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

	deviceSvc := &mocks.FakeDevicesService{}
	deviceSvc.ListDevicesReturns(&model.DeviceList{
		Devices: []*model.Device{},
		Pagination: model.Pagination{
			Page:        1,
			Size:        10,
			TotalItems:  0,
			TotalPages:  1,
			HasNext:     false,
			HasPrevious: false,
		},
		Filters: model.DeviceFilter{Page: 1, Size: 10},
	}, nil)

	app := newTestApp(deviceSvc, newDefaultHealthChecker())
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

	deviceSvc := &mocks.FakeDevicesService{}
	deviceSvc.CreateDeviceStub = func(_ context.Context, name, brand string, state model.State) (*model.Device, error) {
		return model.NewDevice(name, brand, state), nil
	}

	app := newTestApp(deviceSvc, newDefaultHealthChecker())
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

	deviceSvc := &mocks.FakeDevicesService{}
	app := newTestApp(deviceSvc, newDefaultHealthChecker())
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
	deviceSvc := &mocks.FakeDevicesService{}
	deviceSvc.GetDeviceReturns(&model.Device{
		ID:        id,
		Name:      "Test Device",
		Brand:     "Test Brand",
		State:     model.StateAvailable,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil)

	app := newTestApp(deviceSvc, newDefaultHealthChecker())
	handler := handlers.NewDeviceHandler(app)

	req := httptest.NewRequest(http.MethodGet, "/v1/devices/"+id.String(), nil)
	rec := httptest.NewRecorder()

	handler.GetDevice(rec, req, id.UUID, handlers.GetDeviceParams{})

	s.Require().Equal(http.StatusOK, rec.Code)

	var response handlers.DeviceResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	s.Require().NoError(err)
	s.Require().Equal(id.UUID, uuid.UUID(response.Data.Id))
}

func (s *DeviceHandlerTestSuite) TestGetDevice_NotFound() {
	s.T().Parallel()

	deviceSvc := &mocks.FakeDevicesService{}
	deviceSvc.GetDeviceReturns(nil, model.ErrDeviceNotFound)

	app := newTestApp(deviceSvc, newDefaultHealthChecker())
	handler := handlers.NewDeviceHandler(app)

	id := model.NewDeviceID()
	req := httptest.NewRequest(http.MethodGet, "/v1/devices/"+id.String(), nil)
	rec := httptest.NewRecorder()

	handler.GetDevice(rec, req, id.UUID, handlers.GetDeviceParams{})

	s.Require().Equal(http.StatusNotFound, rec.Code)
}

func (s *DeviceHandlerTestSuite) TestDeleteDevice_Success() {
	s.T().Parallel()

	deviceSvc := &mocks.FakeDevicesService{}
	deviceSvc.DeleteDeviceReturns(nil)

	app := newTestApp(deviceSvc, newDefaultHealthChecker())
	handler := handlers.NewDeviceHandler(app)

	id := model.NewDeviceID()
	req := httptest.NewRequest(http.MethodDelete, "/v1/devices/"+id.String(), nil)
	rec := httptest.NewRecorder()

	handler.DeleteDevice(rec, req, id.UUID, handlers.DeleteDeviceParams{})

	s.Require().Equal(http.StatusNoContent, rec.Code)
}

func (s *DeviceHandlerTestSuite) TestDeleteDevice_Errors() {
	s.T().Parallel()

	cases := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{
			name:           "not found",
			err:            model.ErrDeviceNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "in use conflict",
			err:            model.ErrCannotDeleteInUseDevice,
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.T().Parallel()

			deviceSvc := &mocks.FakeDevicesService{}
			deviceSvc.DeleteDeviceReturns(tc.err)

			app := newTestApp(deviceSvc, newDefaultHealthChecker())
			handler := handlers.NewDeviceHandler(app)

			id := model.NewDeviceID()
			req := httptest.NewRequest(http.MethodDelete, "/v1/devices/"+id.String(), nil)
			rec := httptest.NewRecorder()

			handler.DeleteDevice(rec, req, id.UUID, handlers.DeleteDeviceParams{})

			s.Require().Equal(tc.expectedStatus, rec.Code)
		})
	}
}

func (s *DeviceHandlerTestSuite) TestLivenessCheck_Success() {
	s.T().Parallel()

	deviceSvc := &mocks.FakeDevicesService{}
	app := newTestApp(deviceSvc, newDefaultHealthChecker())
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

	deviceSvc := &mocks.FakeDevicesService{}
	app := newTestApp(deviceSvc, newDefaultHealthChecker())
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

	deviceSvc := &mocks.FakeDevicesService{}
	app := newTestApp(deviceSvc, newDefaultHealthChecker())
	handler := handlers.NewDeviceHandler(app)

	id := model.NewDeviceID()
	req := httptest.NewRequest(http.MethodOptions, "/v1/devices/"+id.String(), nil)
	rec := httptest.NewRecorder()

	handler.OptionsDevice(rec, req, id.UUID)

	s.Require().Equal(http.StatusNoContent, rec.Code)
	s.Require().Contains(rec.Header().Get("Allow"), "GET")
	s.Require().Contains(rec.Header().Get("Allow"), "DELETE")
}
