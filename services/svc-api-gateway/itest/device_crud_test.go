//go:build integration

package itest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/stretchr/testify/suite"
)

type DeviceCRUDTestSuite struct {
	BaseTestSuite
}

func TestDeviceCRUDTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	suite.Run(t, new(DeviceCRUDTestSuite))
}

func (s *DeviceCRUDTestSuite) doRequest(method, path string, body map[string]any) (map[string]any, *http.Response) {
	var opts []RequestOption
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		s.Require().NoError(err)
		opts = append(opts, WithBody(bytes.NewReader(bodyBytes)))
	}
	if method == http.MethodPost {
		opts = append(opts, WithIdempotencyKey())
	}

	resp, err := s.Server.DoRequest(s.T().Context(), method, path, opts...)
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
			response, resp := s.doRequest(http.MethodPost, "/v1/devices", tc.body)
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
	resp, err := s.Server.Post(s.T().Context(), "/v1/devices", bytes.NewReader([]byte("invalid json")))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *DeviceCRUDTestSuite) TestGetDevice() {
	deviceID := s.CreateDevice("Test Device", "Test Brand", model.StateAvailable)

	response, resp := s.doRequest(http.MethodGet, s.DevicePath(deviceID), nil)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	data := s.getData(response)
	s.Require().Equal(deviceID, data["id"])
	s.Require().Equal("Test Device", data["name"])
	s.Require().Equal("Test Brand", data["brand"])
}

func (s *DeviceCRUDTestSuite) TestGetDeviceNotFound() {
	nonExistentID := model.NewDeviceID().String()

	_, resp := s.doRequest(http.MethodGet, s.DevicePath(nonExistentID), nil)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *DeviceCRUDTestSuite) TestGetDeviceInvalidID() {
	_, resp := s.doRequest(http.MethodGet, "/v1/devices/invalid-uuid", nil)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *DeviceCRUDTestSuite) TestUpdateDevice() {
	deviceID := s.CreateDevice("Original Name", "Original Brand", model.StateAvailable)

	body := map[string]any{"name": "Updated Name", "brand": "Updated Brand", "state": string(model.StateInactive)}
	response, resp := s.doRequest(http.MethodPut, s.DevicePath(deviceID), body)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	data := s.getData(response)
	s.Require().Equal("Updated Name", data["name"])
	s.Require().Equal("Updated Brand", data["brand"])
	s.Require().Equal(string(model.StateInactive), data["state"])
}

func (s *DeviceCRUDTestSuite) TestUpdateDeviceNotFound() {
	nonExistentID := model.NewDeviceID().String()

	body := map[string]any{"name": "New Name", "brand": "New Brand", "state": string(model.StateAvailable)}
	_, resp := s.doRequest(http.MethodPut, s.DevicePath(nonExistentID), body)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *DeviceCRUDTestSuite) TestPatchDevice() {
	cases := []struct {
		name          string
		initialState  model.State
		body          map[string]any
		expectedName  string
		expectedBrand string
		expectedState string
	}{
		{
			name:          "patch device name only",
			initialState:  model.StateAvailable,
			body:          map[string]any{"name": "Patched Name"},
			expectedName:  "Patched Name",
			expectedBrand: "Test Brand",
			expectedState: string(model.StateAvailable),
		},
		{
			name:          "patch device state only",
			initialState:  model.StateAvailable,
			body:          map[string]any{"state": string(model.StateInactive)},
			expectedName:  "Test Device",
			expectedBrand: "Test Brand",
			expectedState: string(model.StateInactive),
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			deviceID := s.CreateDevice("Test Device", "Test Brand", tc.initialState)

			response, resp := s.doRequest(http.MethodPatch, s.DevicePath(deviceID), tc.body)
			defer resp.Body.Close()

			s.Require().Equal(http.StatusOK, resp.StatusCode)

			data := s.getData(response)
			s.Require().Equal(tc.expectedName, data["name"])
			s.Require().Equal(tc.expectedBrand, data["brand"])
			s.Require().Equal(tc.expectedState, data["state"])
		})
	}
}

func (s *DeviceCRUDTestSuite) TestPatchDeviceNotFound() {
	nonExistentID := model.NewDeviceID().String()

	body := map[string]any{"name": "New Name"}
	_, resp := s.doRequest(http.MethodPatch, s.DevicePath(nonExistentID), body)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *DeviceCRUDTestSuite) TestDeleteDevice() {
	deviceID := s.CreateDevice("Test Device", "Test Brand", model.StateAvailable)

	_, resp := s.doRequest(http.MethodDelete, s.DevicePath(deviceID), nil)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusNoContent, resp.StatusCode)

	_, getResp := s.doRequest(http.MethodGet, s.DevicePath(deviceID), nil)
	defer getResp.Body.Close()
	s.Require().Equal(http.StatusNotFound, getResp.StatusCode)
}

func (s *DeviceCRUDTestSuite) TestDeleteInUseDeviceFails() {
	deviceID := s.CreateDevice("Test Device", "Test Brand", model.StateInUse)

	_, resp := s.doRequest(http.MethodDelete, s.DevicePath(deviceID), nil)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusConflict, resp.StatusCode)
}

func (s *DeviceCRUDTestSuite) TestDeleteDeviceNotFound() {
	nonExistentID := model.NewDeviceID().String()

	_, resp := s.doRequest(http.MethodDelete, s.DevicePath(nonExistentID), nil)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *DeviceCRUDTestSuite) TestHeadDevice() {
	deviceID := s.CreateDevice("Test Device", "Test Brand", model.StateAvailable)

	resp, err := s.Server.Head(s.T().Context(), s.DevicePath(deviceID))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)
}

func (s *DeviceCRUDTestSuite) TestHeadDeviceNotFound() {
	nonExistentID := model.NewDeviceID().String()

	resp, err := s.Server.Head(s.T().Context(), s.DevicePath(nonExistentID))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *DeviceCRUDTestSuite) TestOptionsDevice() {
	deviceID := s.CreateDevice("Test Device", "Test Brand", model.StateAvailable)

	resp, err := s.Server.Options(s.T().Context(), s.DevicePath(deviceID))
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
	deviceID := s.CreateDevice("Original Name", "Original Brand", model.StateInUse)

	body := map[string]any{"name": "Changed Name", "brand": "Changed Brand", "state": string(model.StateInUse)}
	response, resp := s.doRequest(http.MethodPut, s.DevicePath(deviceID), body)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusConflict, resp.StatusCode)
	s.Require().Equal("CONFLICT", response["code"])
}
