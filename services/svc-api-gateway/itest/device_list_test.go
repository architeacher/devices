//go:build integration

package itest

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/stretchr/testify/suite"
)

type DeviceListTestSuite struct {
	BaseTestSuite
}

func TestDeviceListTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	suite.Run(t, new(DeviceListTestSuite))
}

func (s *DeviceListTestSuite) seedDevices() []string {
	devices := []struct {
		name  string
		brand string
		state model.State
	}{
		{name: "iPhone 15", brand: "Apple", state: model.StateAvailable},
		{name: "iPhone 14", brand: "Apple", state: model.StateInUse},
		{name: "Galaxy S24", brand: "Samsung", state: model.StateAvailable},
		{name: "Galaxy S23", brand: "Samsung", state: model.StateInactive},
		{name: "Pixel 8", brand: "Google", state: model.StateAvailable},
	}

	ids := make([]string, 0, len(devices))
	for _, d := range devices {
		id := s.CreateDevice(d.name, d.brand, d.state)
		ids = append(ids, id)
	}

	return ids
}

func (s *DeviceListTestSuite) getDeviceList(path string) ([]any, map[string]any) {
	resp, err := s.Server.Get(s.T().Context(), path)
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var response map[string]any
	err = DecodeJSON(resp.Body, &response)
	s.Require().NoError(err)

	return response["data"].([]any), response["pagination"].(map[string]any)
}

func (s *DeviceListTestSuite) TestListDevices() {
	ids := s.seedDevices()

	data, pagination := s.getDeviceList("/v1/devices")

	s.Require().Equal(len(ids), len(data))
	s.Require().Equal(float64(1), pagination["page"])
	s.Require().Equal(float64(20), pagination["size"])
	s.Require().Equal(float64(len(ids)), pagination["totalItems"])
}

func (s *DeviceListTestSuite) TestListDevicesWithPagination() {
	cases := []struct {
		name              string
		page              int
		size              int
		expectedCount     int
		expectedTotalPage int
		expectedHasNext   bool
		expectedHasPrev   bool
	}{
		{
			name:              "first page size 2",
			page:              1,
			size:              2,
			expectedCount:     2,
			expectedTotalPage: 3,
			expectedHasNext:   true,
			expectedHasPrev:   false,
		},
		{
			name:              "second page size 2",
			page:              2,
			size:              2,
			expectedCount:     2,
			expectedTotalPage: 3,
			expectedHasNext:   true,
			expectedHasPrev:   true,
		},
		{
			name:              "last page size 2",
			page:              3,
			size:              2,
			expectedCount:     1,
			expectedTotalPage: 3,
			expectedHasNext:   false,
			expectedHasPrev:   true,
		},
		{
			name:              "all items in one page",
			page:              1,
			size:              10,
			expectedCount:     5,
			expectedTotalPage: 1,
			expectedHasNext:   false,
			expectedHasPrev:   false,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Require().NoError(s.Server.TruncateDevices(s.T().Context()))
			s.seedDevices()

			path := fmt.Sprintf("/v1/devices?page=%d&size=%d", tc.page, tc.size)
			data, pagination := s.getDeviceList(path)

			s.Require().Equal(tc.expectedCount, len(data))
			s.Require().Equal(float64(tc.page), pagination["page"])
			s.Require().Equal(float64(tc.size), pagination["size"])
			s.Require().Equal(float64(tc.expectedTotalPage), pagination["totalPages"])
			s.Require().Equal(tc.expectedHasNext, pagination["hasNext"])
			s.Require().Equal(tc.expectedHasPrev, pagination["hasPrevious"])
		})
	}
}

func (s *DeviceListTestSuite) TestListDevicesWithBrandFilter() {
	cases := []struct {
		name          string
		brand         string
		expectedCount int
	}{
		{name: "filter by Apple", brand: "Apple", expectedCount: 2},
		{name: "filter by Samsung", brand: "Samsung", expectedCount: 2},
		{name: "filter by Google", brand: "Google", expectedCount: 1},
		{name: "filter by non-existent brand", brand: "NonExistent", expectedCount: 0},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Require().NoError(s.Server.TruncateDevices(s.T().Context()))
			s.seedDevices()

			data, _ := s.getDeviceList(fmt.Sprintf("/v1/devices?brand=%s", tc.brand))
			s.Require().Equal(tc.expectedCount, len(data))
		})
	}
}

func (s *DeviceListTestSuite) TestListDevicesWithStateFilter() {
	cases := []struct {
		name          string
		state         string
		expectedCount int
	}{
		{name: "filter by available", state: "available", expectedCount: 3},
		{name: "filter by in-use", state: "in-use", expectedCount: 1},
		{name: "filter by inactive", state: "inactive", expectedCount: 1},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Require().NoError(s.Server.TruncateDevices(s.T().Context()))
			s.seedDevices()

			data, _ := s.getDeviceList(fmt.Sprintf("/v1/devices?state=%s", tc.state))
			s.Require().Equal(tc.expectedCount, len(data))
		})
	}
}

func (s *DeviceListTestSuite) TestListDevicesWithCombinedFilters() {
	cases := []struct {
		name          string
		brand         string
		state         string
		expectedCount int
	}{
		{name: "Apple and available", brand: "Apple", state: "available", expectedCount: 1},
		{name: "Samsung and available", brand: "Samsung", state: "available", expectedCount: 1},
		{name: "Apple and inactive", brand: "Apple", state: "inactive", expectedCount: 0},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Require().NoError(s.Server.TruncateDevices(s.T().Context()))
			s.seedDevices()

			path := fmt.Sprintf("/v1/devices?brand=%s&state=%s", tc.brand, tc.state)
			data, _ := s.getDeviceList(path)
			s.Require().Equal(tc.expectedCount, len(data))
		})
	}
}

func (s *DeviceListTestSuite) TestHeadDevices() {
	ids := s.seedDevices()

	resp, err := s.Server.Head(s.T().Context(), "/v1/devices")
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Require().Equal(fmt.Sprintf("%d", len(ids)), resp.Header.Get("X-Total-Count"))
}

func (s *DeviceListTestSuite) TestOptionsDevices() {
	resp, err := s.Server.Options(s.T().Context(), "/v1/devices")
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusNoContent, resp.StatusCode)

	allow := resp.Header.Get("Allow")
	s.Require().Contains(allow, "GET")
	s.Require().Contains(allow, "POST")
	s.Require().Contains(allow, "HEAD")
	s.Require().Contains(allow, "OPTIONS")
}

func (s *DeviceListTestSuite) TestListEmptyDevices() {
	data, pagination := s.getDeviceList("/v1/devices")

	s.Require().Empty(data)
	s.Require().Equal(float64(0), pagination["totalItems"])
}

func (s *DeviceListTestSuite) TestDeviceResponseContainsLinks() {
	deviceID := s.CreateDevice("Test Device", "Test Brand", model.StateAvailable)

	data, _ := s.getDeviceList("/v1/devices")

	s.Require().Len(data, 1)

	deviceData := data[0].(map[string]any)
	links := deviceData["links"].(map[string]any)
	s.Require().NotEmpty(links["self"])
	s.Require().Contains(links["self"], deviceID)
}
