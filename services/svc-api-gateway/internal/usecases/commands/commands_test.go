package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/pkg/metrics/noop"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/usecases/commands"
	"github.com/stretchr/testify/suite"
	otelNoop "go.opentelemetry.io/otel/trace/noop"
)

type mockDeviceService struct {
	createDeviceFn func(ctx context.Context, name, brand string, state model.State) (*model.Device, error)
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

func (m *mockDeviceService) GetDevice(_ context.Context, id model.DeviceID) (*model.Device, error) {
	return &model.Device{ID: id}, nil
}

func (m *mockDeviceService) ListDevices(_ context.Context, filter model.DeviceFilter) (*model.DeviceList, error) {
	return &model.DeviceList{Filters: filter}, nil
}

func (m *mockDeviceService) UpdateDevice(ctx context.Context, id model.DeviceID, name, brand string, state model.State) (*model.Device, error) {
	if m.updateDeviceFn != nil {
		return m.updateDeviceFn(ctx, id, name, brand, state)
	}

	return &model.Device{ID: id, Name: name, Brand: brand, State: state}, nil
}

func (m *mockDeviceService) PatchDevice(ctx context.Context, id model.DeviceID, updates map[string]any) (*model.Device, error) {
	if m.patchDeviceFn != nil {
		return m.patchDeviceFn(ctx, id, updates)
	}

	return &model.Device{ID: id}, nil
}

func (m *mockDeviceService) DeleteDevice(ctx context.Context, id model.DeviceID) error {
	if m.deleteDeviceFn != nil {
		return m.deleteDeviceFn(ctx, id)
	}

	return nil
}

type CreateDeviceCommandTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestCreateDeviceCommandTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(CreateDeviceCommandTestSuite))
}

func (s *CreateDeviceCommandTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *CreateDeviceCommandTestSuite) TestHandle_Success() {
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
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			svc := &mockDeviceService{}
			handler := commands.NewCreateDeviceCommandHandler(svc, logger.NewTestLogger(), otelNoop.NewTracerProvider(), noop.NewMetricsClient())

			cmd := commands.CreateDeviceCommand{
				Name:  tc.dName,
				Brand: tc.brand,
				State: tc.state,
			}

			result, err := handler.Handle(s.ctx, cmd)

			s.Require().NoError(err)
			s.Require().NotNil(result)
			s.Require().Equal(tc.dName, result.Name)
			s.Require().Equal(tc.brand, result.Brand)
			s.Require().Equal(tc.state, result.State)
		})
	}
}

func (s *CreateDeviceCommandTestSuite) TestHandle_ServiceError() {
	s.T().Parallel()

	expectedErr := errors.New("service error")
	svc := &mockDeviceService{
		createDeviceFn: func(_ context.Context, _, _ string, _ model.State) (*model.Device, error) {
			return nil, expectedErr
		},
	}
	handler := commands.NewCreateDeviceCommandHandler(svc, logger.NewTestLogger(), otelNoop.NewTracerProvider(), noop.NewMetricsClient())

	cmd := commands.CreateDeviceCommand{
		Name:  "Test",
		Brand: "Brand",
		State: model.StateAvailable,
	}

	result, err := handler.Handle(s.ctx, cmd)

	s.Require().Error(err)
	s.Require().Nil(result)
}

type UpdateDeviceCommandTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestUpdateDeviceCommandTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(UpdateDeviceCommandTestSuite))
}

func (s *UpdateDeviceCommandTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *UpdateDeviceCommandTestSuite) TestHandle_Success() {
	s.T().Parallel()

	svc := &mockDeviceService{}
	handler := commands.NewUpdateDeviceCommandHandler(svc, logger.NewTestLogger(), otelNoop.NewTracerProvider(), noop.NewMetricsClient())

	id := model.NewDeviceID()
	cmd := commands.UpdateDeviceCommand{
		ID:    id,
		Name:  "Updated",
		Brand: "Updated Brand",
		State: model.StateInUse,
	}

	result, err := handler.Handle(s.ctx, cmd)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().Equal(id, result.ID)
	s.Require().Equal(cmd.Name, result.Name)
	s.Require().Equal(cmd.Brand, result.Brand)
	s.Require().Equal(cmd.State, result.State)
}

type DeleteDeviceCommandTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestDeleteDeviceCommandTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(DeleteDeviceCommandTestSuite))
}

func (s *DeleteDeviceCommandTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *DeleteDeviceCommandTestSuite) TestHandle_Success() {
	s.T().Parallel()

	svc := &mockDeviceService{}
	handler := commands.NewDeleteDeviceCommandHandler(svc, logger.NewTestLogger(), otelNoop.NewTracerProvider(), noop.NewMetricsClient())

	id := model.NewDeviceID()
	cmd := commands.DeleteDeviceCommand{ID: id}

	result, err := handler.Handle(s.ctx, cmd)

	s.Require().NoError(err)
	s.Require().True(result.Success)
}

func (s *DeleteDeviceCommandTestSuite) TestHandle_ServiceError() {
	s.T().Parallel()

	expectedErr := errors.New("cannot delete")
	svc := &mockDeviceService{
		deleteDeviceFn: func(_ context.Context, _ model.DeviceID) error {
			return expectedErr
		},
	}
	handler := commands.NewDeleteDeviceCommandHandler(svc, logger.NewTestLogger(), otelNoop.NewTracerProvider(), noop.NewMetricsClient())

	cmd := commands.DeleteDeviceCommand{ID: model.NewDeviceID()}

	result, err := handler.Handle(s.ctx, cmd)

	s.Require().Error(err)
	s.Require().False(result.Success)
}
