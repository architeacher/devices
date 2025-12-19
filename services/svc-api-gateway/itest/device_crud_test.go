package itest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/stretchr/testify/suite"
)

type DeviceCRUDTestSuite struct {
	suite.Suite
}

func TestDeviceCRUDTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(DeviceCRUDTestSuite))
}

func (s *DeviceCRUDTestSuite) devicePath(id string) string {
	return fmt.Sprintf("/v1/devices/%s", id)
}

func (s *DeviceCRUDTestSuite) setupDevice(server *TestServer, state model.State) *model.Device {
	device := &model.Device{
		ID:        model.NewDeviceID(),
		Name:      "Test Device",
		Brand:     "Test Brand",
		State:     state,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	server.DeviceService.AddDevice(device)

	return device
}

func (s *DeviceCRUDTestSuite) getDeviceID(server *TestServer, setup bool, state model.State) string {
	if setup {
		return s.setupDevice(server, state).ID.String()
	}

	return model.NewDeviceID().String()
}

func (s *DeviceCRUDTestSuite) doRequest(server *TestServer, method, path string, body map[string]any) (map[string]any, *http.Response) {
	var opts []RequestOption
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		s.Require().NoError(err)
		opts = append(opts, WithBody(bytes.NewReader(bodyBytes)))
	}
	if method == http.MethodPost {
		opts = append(opts, WithIdempotencyKey())
	}

	resp, err := server.DoRequest(s.T().Context(), method, path, opts...)
	s.Require().NoError(err)

	var response map[string]any
	if resp.StatusCode != http.StatusNoContent {
		_ = DecodeJSON(resp.Body, &response)
	}

	return response, resp
}

func (s *DeviceCRUDTestSuite) getData(response map[string]any) map[string]any {
	return response["data"].(map[string]any)
}

func (s *DeviceCRUDTestSuite) TestCreateDevice() {
	cases := []struct {
		name           string
		body           map[string]any
		expectedStatus int
		expectedState  string
		checkLocation  bool
	}{
		{
			name:           "create device with default state",
			body:           map[string]any{"name": "iPhone 15", "brand": "Apple"},
			expectedStatus: http.StatusCreated,
			expectedState:  string(model.StateAvailable),
			checkLocation:  true,
		},
		{
			name:           "create device with explicit state",
			body:           map[string]any{"name": "Galaxy S24", "brand": "Samsung", "state": string(model.StateInactive)},
			expectedStatus: http.StatusCreated,
			expectedState:  string(model.StateInactive),
			checkLocation:  true,
		},
		{
			name:           "create device with in-use state",
			body:           map[string]any{"name": "Pixel 8", "brand": "Google", "state": string(model.StateInUse)},
			expectedStatus: http.StatusCreated,
			expectedState:  string(model.StateInUse),
			checkLocation:  true,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			server := NewTestServer()
			defer server.Close()

			response, resp := s.doRequest(server, http.MethodPost, "/v1/devices", tc.body)
			defer resp.Body.Close()

			s.Require().Equal(tc.expectedStatus, resp.StatusCode)

			if tc.checkLocation {
				s.Require().NotEmpty(resp.Header.Get("Location"))
			}

			data := s.getData(response)
			s.Require().NotEmpty(data["id"])
			s.Require().Equal(tc.body["name"], data["name"])
			s.Require().Equal(tc.body["brand"], data["brand"])
			s.Require().Equal(tc.expectedState, data["state"])
		})
	}
}

func (s *DeviceCRUDTestSuite) TestCreateDeviceInvalidJSON() {
	server := NewTestServer()
	defer server.Close()

	resp, err := server.Post(s.T().Context(), "/v1/devices", bytes.NewReader([]byte("invalid json")))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *DeviceCRUDTestSuite) TestGetDevice() {
	cases := []struct {
		name           string
		setupDevice    bool
		expectedStatus int
		expectFound    bool
	}{
		{name: "get existing device", setupDevice: true, expectedStatus: http.StatusOK, expectFound: true},
		{name: "get non-existent device", setupDevice: false, expectedStatus: http.StatusNotFound, expectFound: false},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			server := NewTestServer()
			defer server.Close()

			deviceID := s.getDeviceID(server, tc.setupDevice, model.StateAvailable)

			response, resp := s.doRequest(server, http.MethodGet, s.devicePath(deviceID), nil)
			defer resp.Body.Close()

			s.Require().Equal(tc.expectedStatus, resp.StatusCode)

			if tc.expectFound {
				data := s.getData(response)
				s.Require().Equal(deviceID, data["id"])
				s.Require().Equal("Test Device", data["name"])
				s.Require().Equal("Test Brand", data["brand"])
			}
		})
	}
}

func (s *DeviceCRUDTestSuite) TestGetDeviceInvalidID() {
	server := NewTestServer()
	defer server.Close()

	_, resp := s.doRequest(server, http.MethodGet, "/v1/devices/invalid-uuid", nil)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *DeviceCRUDTestSuite) TestUpdateDevice() {
	cases := []struct {
		name           string
		setupDevice    bool
		body           map[string]any
		expectedStatus int
	}{
		{
			name:           "update device successfully",
			setupDevice:    true,
			body:           map[string]any{"name": "Updated Name", "brand": "Updated Brand", "state": string(model.StateInactive)},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "update non-existent device",
			setupDevice:    false,
			body:           map[string]any{"name": "New Name", "brand": "New Brand", "state": string(model.StateAvailable)},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			server := NewTestServer()
			defer server.Close()

			deviceID := s.getDeviceID(server, tc.setupDevice, model.StateAvailable)

			response, resp := s.doRequest(server, http.MethodPut, s.devicePath(deviceID), tc.body)
			defer resp.Body.Close()

			s.Require().Equal(tc.expectedStatus, resp.StatusCode)

			if tc.expectedStatus == http.StatusOK {
				data := s.getData(response)
				s.Require().Equal(tc.body["name"], data["name"])
				s.Require().Equal(tc.body["brand"], data["brand"])
				s.Require().Equal(tc.body["state"], data["state"])
			}
		})
	}
}

func (s *DeviceCRUDTestSuite) TestPatchDevice() {
	cases := []struct {
		name           string
		setupDevice    bool
		body           map[string]any
		expectedStatus int
		expectedName   string
		expectedBrand  string
		expectedState  string
	}{
		{
			name:           "patch device name only",
			setupDevice:    true,
			body:           map[string]any{"name": "Patched Name"},
			expectedStatus: http.StatusOK,
			expectedName:   "Patched Name",
			expectedBrand:  "Test Brand",
			expectedState:  string(model.StateAvailable),
		},
		{
			name:           "patch device state only",
			setupDevice:    true,
			body:           map[string]any{"state": string(model.StateInactive)},
			expectedStatus: http.StatusOK,
			expectedName:   "Test Device",
			expectedBrand:  "Test Brand",
			expectedState:  string(model.StateInactive),
		},
		{
			name:           "patch non-existent device",
			setupDevice:    false,
			body:           map[string]any{"name": "New Name"},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			server := NewTestServer()
			defer server.Close()

			deviceID := s.getDeviceID(server, tc.setupDevice, model.StateAvailable)

			response, resp := s.doRequest(server, http.MethodPatch, s.devicePath(deviceID), tc.body)
			defer resp.Body.Close()

			s.Require().Equal(tc.expectedStatus, resp.StatusCode)

			if tc.expectedStatus == http.StatusOK {
				data := s.getData(response)
				s.Require().Equal(tc.expectedName, data["name"])
				s.Require().Equal(tc.expectedBrand, data["brand"])
				s.Require().Equal(tc.expectedState, data["state"])
			}
		})
	}
}

func (s *DeviceCRUDTestSuite) TestDeleteDevice() {
	cases := []struct {
		name           string
		deviceState    model.State
		setupDevice    bool
		expectedStatus int
	}{
		{name: "delete available device", deviceState: model.StateAvailable, setupDevice: true, expectedStatus: http.StatusNoContent},
		{name: "delete in-use device should fail", deviceState: model.StateInUse, setupDevice: true, expectedStatus: http.StatusConflict},
		{name: "delete non-existent device", setupDevice: false, expectedStatus: http.StatusNotFound},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			server := NewTestServer()
			defer server.Close()

			deviceID := s.getDeviceID(server, tc.setupDevice, tc.deviceState)

			_, resp := s.doRequest(server, http.MethodDelete, s.devicePath(deviceID), nil)
			defer resp.Body.Close()

			s.Require().Equal(tc.expectedStatus, resp.StatusCode)
		})
	}
}

func (s *DeviceCRUDTestSuite) TestHeadDevice() {
	cases := []struct {
		name           string
		setupDevice    bool
		expectedStatus int
	}{
		{name: "head existing device", setupDevice: true, expectedStatus: http.StatusOK},
		{name: "head non-existent device", setupDevice: false, expectedStatus: http.StatusNotFound},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			server := NewTestServer()
			defer server.Close()

			deviceID := s.getDeviceID(server, tc.setupDevice, model.StateAvailable)

			resp, err := server.Head(s.T().Context(), s.devicePath(deviceID))
			s.Require().NoError(err)
			defer resp.Body.Close()

			s.Require().Equal(tc.expectedStatus, resp.StatusCode)
		})
	}
}

func (s *DeviceCRUDTestSuite) TestOptionsDevice() {
	server := NewTestServer()
	defer server.Close()

	deviceID := s.getDeviceID(server, true, model.StateAvailable)

	resp, err := server.Options(s.T().Context(), s.devicePath(deviceID))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusNoContent, resp.StatusCode)

	allow := resp.Header.Get("Allow")
	s.Require().Contains(allow, "GET")
	s.Require().Contains(allow, "PUT")
	s.Require().Contains(allow, "PATCH")
	s.Require().Contains(allow, "DELETE")
	s.Require().Contains(allow, "HEAD")
	s.Require().Contains(allow, "OPTIONS")
}

func (s *DeviceCRUDTestSuite) TestUpdateInUseDeviceConflict() {
	server := NewTestServer()
	defer server.Close()

	deviceID := s.getDeviceID(server, true, model.StateInUse)

	body := map[string]any{"name": "Changed Name", "brand": "Changed Brand", "state": string(model.StateInUse)}

	response, resp := s.doRequest(server, http.MethodPut, s.devicePath(deviceID), body)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusConflict, resp.StatusCode)
	s.Require().Equal("CONFLICT", response["code"])
}
