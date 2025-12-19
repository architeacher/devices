package itest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/stretchr/testify/suite"
)

type ErrorScenariosTestSuite struct {
	suite.Suite
}

func TestErrorScenariosTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ErrorScenariosTestSuite))
}

func (s *ErrorScenariosTestSuite) devicePath(id string) string {
	return fmt.Sprintf("/v1/devices/%s", id)
}

func (s *ErrorScenariosTestSuite) setupInUseDevice(server *TestServer) *model.Device {
	device := &model.Device{
		ID:        model.NewDeviceID(),
		Name:      "In-Use Device",
		Brand:     "Brand",
		State:     model.StateInUse,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	server.DeviceService.AddDevice(device)

	return device
}

func (s *ErrorScenariosTestSuite) TestInvalidJSONBody() {
	cases := []struct {
		name           string
		method         string
		pathFn         func() string
		body           string
		expectedStatus int
	}{
		{
			name:           "create device with invalid JSON",
			method:         http.MethodPost,
			pathFn:         func() string { return "/v1/devices" },
			body:           "not valid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "update device with invalid JSON",
			method:         http.MethodPut,
			pathFn:         func() string { return "/v1/devices/" + model.NewDeviceID().String() },
			body:           "{broken json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "patch device with invalid JSON",
			method:         http.MethodPatch,
			pathFn:         func() string { return "/v1/devices/" + model.NewDeviceID().String() },
			body:           `{"name": }`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			server := NewTestServer()
			defer server.Close()

			var opts []RequestOption
			opts = append(opts, WithBody(bytes.NewReader([]byte(tc.body))))
			if tc.method == http.MethodPost {
				opts = append(opts, WithIdempotencyKey())
			}

			resp, err := server.DoRequest(s.T().Context(), tc.method, tc.pathFn(), opts...)
			s.Require().NoError(err)
			defer resp.Body.Close()

			s.Require().Equal(tc.expectedStatus, resp.StatusCode)
		})
	}
}

func (s *ErrorScenariosTestSuite) TestInvalidDeviceID() {
	invalidIDs := []string{
		"invalid-uuid",
		"12345",
		"not-a-valid-uuid-format",
	}

	methods := []string{http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead}

	for _, invalidID := range invalidIDs {
		for _, method := range methods {
			testName := fmt.Sprintf("%s with invalid ID: %s", method, invalidID)
			s.Run(testName, func() {
				server := NewTestServer()
				defer server.Close()

				path := s.devicePath(invalidID)

				var opts []RequestOption
				if method == http.MethodPut || method == http.MethodPatch {
					body := []byte(`{"name": "test", "brand": "test", "state": "available"}`)
					opts = append(opts, WithBody(bytes.NewReader(body)))
				}

				resp, err := server.DoRequest(s.T().Context(), method, path, opts...)
				s.Require().NoError(err)
				defer resp.Body.Close()

				s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
			})
		}
	}
}

func (s *ErrorScenariosTestSuite) TestDeviceNotFound() {
	cases := []struct {
		name           string
		method         string
		body           []byte
		expectedStatus int
	}{
		{
			name:           "get non-existent device",
			method:         http.MethodGet,
			body:           nil,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "update non-existent device",
			method:         http.MethodPut,
			body:           []byte(`{"name": "test", "brand": "test", "state": "available"}`),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "patch non-existent device",
			method:         http.MethodPatch,
			body:           []byte(`{"name": "test"}`),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "delete non-existent device",
			method:         http.MethodDelete,
			body:           nil,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "head non-existent device",
			method:         http.MethodHead,
			body:           nil,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			server := NewTestServer()
			defer server.Close()

			nonExistentID := model.NewDeviceID().String()
			path := s.devicePath(nonExistentID)

			var opts []RequestOption
			if tc.body != nil {
				opts = append(opts, WithBody(bytes.NewReader(tc.body)))
			}

			resp, err := server.DoRequest(s.T().Context(), tc.method, path, opts...)
			s.Require().NoError(err)
			defer resp.Body.Close()

			s.Require().Equal(tc.expectedStatus, resp.StatusCode)
		})
	}
}

func (s *ErrorScenariosTestSuite) TestBusinessRuleViolations() {
	cases := []struct {
		name           string
		method         string
		bodyFn         func(device *model.Device) []byte
		expectedStatus int
		expectedCode   string
	}{
		{
			name:   "cannot update name of in-use device",
			method: http.MethodPut,
			bodyFn: func(device *model.Device) []byte {
				return []byte(fmt.Sprintf(`{
					"name": "Changed Name",
					"brand": "%s",
					"state": "in-use"
				}`, device.Brand))
			},
			expectedStatus: http.StatusConflict,
			expectedCode:   "CONFLICT",
		},
		{
			name:   "cannot update brand of in-use device",
			method: http.MethodPut,
			bodyFn: func(device *model.Device) []byte {
				return []byte(fmt.Sprintf(`{
					"name": "%s",
					"brand": "Changed Brand",
					"state": "in-use"
				}`, device.Name))
			},
			expectedStatus: http.StatusConflict,
			expectedCode:   "CONFLICT",
		},
		{
			name:   "cannot patch name of in-use device",
			method: http.MethodPatch,
			bodyFn: func(_ *model.Device) []byte {
				return []byte(`{"name": "Changed Name"}`)
			},
			expectedStatus: http.StatusConflict,
			expectedCode:   "CONFLICT",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			server := NewTestServer()
			defer server.Close()

			device := s.setupInUseDevice(server)
			body := tc.bodyFn(device)

			resp, err := server.DoRequest(
				s.T().Context(),
				tc.method,
				s.devicePath(device.ID.String()),
				WithBody(bytes.NewReader(body)),
			)
			s.Require().NoError(err)
			defer resp.Body.Close()

			s.Require().Equal(tc.expectedStatus, resp.StatusCode)

			var response map[string]any
			s.Require().NoError(DecodeJSON(resp.Body, &response))
			s.Require().Equal(tc.expectedCode, response["code"])
		})
	}
}

func (s *ErrorScenariosTestSuite) TestCannotDeleteInUseDevice() {
	server := NewTestServer()
	defer server.Close()

	device := s.setupInUseDevice(server)

	resp, err := server.Delete(s.T().Context(), s.devicePath(device.ID.String()))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusConflict, resp.StatusCode)
}

func (s *ErrorScenariosTestSuite) TestInternalServerErrors() {
	server := NewTestServer()
	defer server.Close()

	server.DeviceService.CreateDeviceFn = func(_ context.Context, _, _ string, _ model.State) (*model.Device, error) {
		return nil, errors.New("internal error")
	}

	body := []byte(`{"name": "Test", "brand": "Brand"}`)

	resp, err := server.Post(s.T().Context(), "/v1/devices", bytes.NewReader(body))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusInternalServerError, resp.StatusCode)

	var response map[string]any
	s.Require().NoError(DecodeJSON(resp.Body, &response))
	s.Require().Equal("INTERNAL_ERROR", response["code"])
}

func (s *ErrorScenariosTestSuite) TestListDevicesInternalError() {
	server := NewTestServer()
	defer server.Close()

	server.DeviceService.ListDevicesFn = func(_ context.Context, _ model.DeviceFilter) (*model.DeviceList, error) {
		return nil, errors.New("database connection failed")
	}

	resp, err := server.Get(s.T().Context(), "/v1/devices")
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusInternalServerError, resp.StatusCode)

	var response map[string]any
	s.Require().NoError(DecodeJSON(resp.Body, &response))
	s.Require().Equal("INTERNAL_ERROR", response["code"])
}

func (s *ErrorScenariosTestSuite) TestErrorResponseFormat() {
	server := NewTestServer()
	defer server.Close()

	resp, err := server.Get(s.T().Context(), s.devicePath(model.NewDeviceID().String()))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
	s.Require().Contains(resp.Header.Get("Content-Type"), "application/json")

	var response map[string]any
	s.Require().NoError(DecodeJSON(resp.Body, &response))

	s.Require().Contains(response, "code")
	s.Require().Contains(response, "message")
	s.Require().Contains(response, "timestamp")

	s.Require().IsType("", response["code"])
	s.Require().IsType("", response["message"])
	s.Require().IsType("", response["timestamp"])
}

func (s *ErrorScenariosTestSuite) TestUndefinedMethodReturnsNotFound() {
	server := NewTestServer()
	defer server.Close()

	resp, err := server.DoRequest(s.T().Context(), "TRACE", "/v1/devices")
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *ErrorScenariosTestSuite) TestCanUpdateStateOfInUseDevice() {
	server := NewTestServer()
	defer server.Close()

	device := s.setupInUseDevice(server)

	body := []byte(fmt.Sprintf(`{
		"name": "%s",
		"brand": "%s",
		"state": "available"
	}`, device.Name, device.Brand))

	resp, err := server.Put(s.T().Context(), s.devicePath(device.ID.String()), bytes.NewReader(body))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var response map[string]any
	s.Require().NoError(DecodeJSON(resp.Body, &response))

	data := response["data"].(map[string]any)
	s.Require().Equal("available", data["state"])
}
