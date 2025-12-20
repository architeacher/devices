//go:build integration

package itest

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/stretchr/testify/suite"
)

type ErrorScenariosTestSuite struct {
	BaseTestSuite
}

func TestErrorScenariosTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	suite.Run(t, new(ErrorScenariosTestSuite))
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
			var opts []RequestOption
			opts = append(opts, WithBody(bytes.NewReader([]byte(tc.body))))
			if tc.method == http.MethodPost {
				opts = append(opts, WithIdempotencyKey())
			}

			resp, err := s.Server.DoRequest(s.T().Context(), tc.method, tc.pathFn(), opts...)
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
				path := s.DevicePath(invalidID)

				var opts []RequestOption
				if method == http.MethodPut || method == http.MethodPatch {
					body := []byte(`{"name": "test", "brand": "test", "state": "available"}`)
					opts = append(opts, WithBody(bytes.NewReader(body)))
				}

				resp, err := s.Server.DoRequest(s.T().Context(), method, path, opts...)
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
			nonExistentID := model.NewDeviceID().String()
			path := s.DevicePath(nonExistentID)

			var opts []RequestOption
			if tc.body != nil {
				opts = append(opts, WithBody(bytes.NewReader(tc.body)))
			}

			resp, err := s.Server.DoRequest(s.T().Context(), tc.method, path, opts...)
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
		bodyFn         func(deviceID, brand string) []byte
		expectedStatus int
		expectedCode   string
	}{
		{
			name:   "cannot update name of in-use device",
			method: http.MethodPut,
			bodyFn: func(_, brand string) []byte {
				return []byte(fmt.Sprintf(`{
					"name": "Changed Name",
					"brand": "%s",
					"state": "in-use"
				}`, brand))
			},
			expectedStatus: http.StatusConflict,
			expectedCode:   "CONFLICT",
		},
		{
			name:   "cannot update brand of in-use device",
			method: http.MethodPut,
			bodyFn: func(_, _ string) []byte {
				return []byte(`{
					"name": "In-Use Device",
					"brand": "Changed Brand",
					"state": "in-use"
				}`)
			},
			expectedStatus: http.StatusConflict,
			expectedCode:   "CONFLICT",
		},
		{
			name:   "cannot patch name of in-use device",
			method: http.MethodPatch,
			bodyFn: func(_, _ string) []byte {
				return []byte(`{"name": "Changed Name"}`)
			},
			expectedStatus: http.StatusConflict,
			expectedCode:   "CONFLICT",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			deviceID := s.CreateDevice("In-Use Device", "Brand", model.StateInUse)
			body := tc.bodyFn(deviceID, "Brand")

			resp, err := s.Server.DoRequest(
				s.T().Context(),
				tc.method,
				s.DevicePath(deviceID),
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
	deviceID := s.CreateDevice("In-Use Device", "Brand", model.StateInUse)

	resp, err := s.Server.Delete(s.T().Context(), s.DevicePath(deviceID))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusConflict, resp.StatusCode)
}

func (s *ErrorScenariosTestSuite) TestErrorResponseFormat() {
	resp, err := s.Server.Get(s.T().Context(), s.DevicePath(model.NewDeviceID().String()))
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
	resp, err := s.Server.DoRequest(s.T().Context(), "TRACE", "/v1/devices")
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *ErrorScenariosTestSuite) TestCanUpdateStateOfInUseDevice() {
	deviceID := s.CreateDevice("In-Use Device", "Brand", model.StateInUse)

	body := []byte(`{
		"name": "In-Use Device",
		"brand": "Brand",
		"state": "available"
	}`)

	resp, err := s.Server.Put(s.T().Context(), s.DevicePath(deviceID), bytes.NewReader(body))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var response map[string]any
	s.Require().NoError(DecodeJSON(resp.Body, &response))

	data := response["data"].(map[string]any)
	s.Require().Equal("available", data["state"])
}
