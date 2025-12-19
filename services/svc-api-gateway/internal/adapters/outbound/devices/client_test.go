package devices_test

import (
	"context"
	"testing"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/outbound/devices"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/stretchr/testify/suite"
)

type ClientTestSuite struct {
	suite.Suite
	client *devices.Client
	ctx    context.Context
}

func TestClientTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ClientTestSuite))
}

func (s *ClientTestSuite) SetupTest() {
	s.client = devices.NewClient()
	s.ctx = context.Background()
}

func (s *ClientTestSuite) TestNewClient() {
	s.T().Parallel()

	client := devices.NewClient()

	s.Require().NotNil(client)
}

func (s *ClientTestSuite) TestCreateDevice() {
	s.T().Parallel()

	cases := []struct {
		name  string
		dName string
		brand string
		state model.State
	}{
		{
			name:  "create available device",
			dName: "iPhone",
			brand: "Apple",
			state: model.StateAvailable,
		},
		{
			name:  "create in-use device",
			dName: "Pixel",
			brand: "Google",
			state: model.StateInUse,
		},
		{
			name:  "create inactive device",
			dName: "Galaxy",
			brand: "Samsung",
			state: model.StateInactive,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			device, err := s.client.CreateDevice(s.ctx, tc.dName, tc.brand, tc.state)

			s.Require().NoError(err)
			s.Require().NotNil(device)
			s.Require().False(device.ID.IsZero())
			s.Require().Equal(tc.dName, device.Name)
			s.Require().Equal(tc.brand, device.Brand)
			s.Require().Equal(tc.state, device.State)
			s.Require().False(device.CreatedAt.IsZero())
			s.Require().False(device.UpdatedAt.IsZero())
		})
	}
}

func (s *ClientTestSuite) TestGetDevice() {
	s.T().Parallel()

	id := model.NewDeviceID()

	device, err := s.client.GetDevice(s.ctx, id)

	s.Require().NoError(err)
	s.Require().NotNil(device)
	s.Require().Equal(id, device.ID)
	s.Require().Equal(model.StateAvailable, device.State)
}

func (s *ClientTestSuite) TestListDevices() {
	s.T().Parallel()

	filter := model.DefaultDeviceFilter()

	result, err := s.client.ListDevices(s.ctx, filter)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().Empty(result.Devices)
	s.Require().Equal(filter.Page, result.Pagination.Page)
	s.Require().Equal(filter.Size, result.Pagination.Size)
	s.Require().Equal(uint(0), result.Pagination.TotalItems)
	s.Require().Equal(uint(1), result.Pagination.TotalPages)
	s.Require().False(result.Pagination.HasNext)
	s.Require().False(result.Pagination.HasPrevious)
}

func (s *ClientTestSuite) TestUpdateDevice() {
	s.T().Parallel()

	id := model.NewDeviceID()
	name := "Updated Device"
	brand := "Updated Brand"
	state := model.StateInUse

	device, err := s.client.UpdateDevice(s.ctx, id, name, brand, state)

	s.Require().NoError(err)
	s.Require().NotNil(device)
	s.Require().Equal(id, device.ID)
	s.Require().Equal(name, device.Name)
	s.Require().Equal(brand, device.Brand)
	s.Require().Equal(state, device.State)
}

func (s *ClientTestSuite) TestPatchDevice() {
	s.T().Parallel()

	id := model.NewDeviceID()
	updates := map[string]any{"name": "Patched"}

	device, err := s.client.PatchDevice(s.ctx, id, updates)

	s.Require().NoError(err)
	s.Require().NotNil(device)
	s.Require().Equal(id, device.ID)
}

func (s *ClientTestSuite) TestDeleteDevice() {
	s.T().Parallel()

	id := model.NewDeviceID()

	err := s.client.DeleteDevice(s.ctx, id)

	s.Require().NoError(err)
}
