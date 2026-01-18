package repos_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/repos"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/infrastructure"
	"github.com/stretchr/testify/suite"
)

type DevicesCacheRepositoryTestSuite struct {
	suite.Suite
	miniRedis   *miniredis.Miniredis
	keydbClient *infrastructure.KeydbClient
	repo        *repos.DevicesCacheRepository
}

func TestDevicesCacheRepositoryTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(DevicesCacheRepositoryTestSuite))
}

func (s *DevicesCacheRepositoryTestSuite) SetupTest() {
	var err error
	s.miniRedis, err = miniredis.Run()
	s.Require().NoError(err)

	cfg := config.Cache{
		Address:      s.miniRedis.Addr(),
		Password:     "",
		DB:           0,
		PoolSize:     5,
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
	}

	s.keydbClient = infrastructure.NewKeyDBClient(cfg, logger.NewTestLogger())
	s.repo = repos.NewDevicesCacheRepository(s.keydbClient, logger.NewTestLogger())
}

func (s *DevicesCacheRepositoryTestSuite) TearDownTest() {
	if s.keydbClient != nil {
		_ = s.keydbClient.Close()
	}
	if s.miniRedis != nil {
		s.miniRedis.Close()
	}
}

func (s *DevicesCacheRepositoryTestSuite) TestNewDevicesCacheRepository() {
	s.Require().NotNil(s.repo)
}

func (s *DevicesCacheRepositoryTestSuite) TestGetDevice_NotCached() {
	ctx := context.Background()
	id := model.NewDeviceID()

	result, err := s.repo.GetDevice(ctx, id)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().False(result.Hit)
	s.Require().Nil(result.Data)
}

func (s *DevicesCacheRepositoryTestSuite) TestSetAndGetDevice() {
	ctx := context.Background()
	device := model.NewDevice("Test Device", "Test Brand", model.StateAvailable)
	ttl := time.Hour

	err := s.repo.SetDevice(ctx, device, ttl)
	s.Require().NoError(err)

	result, err := s.repo.GetDevice(ctx, device.ID)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().True(result.Hit)
	s.Require().NotNil(result.Data)
	s.Require().Equal(device.ID, result.Data.ID)
	s.Require().Equal(device.Name, result.Data.Name)
	s.Require().Equal(device.Brand, result.Data.Brand)
	s.Require().Equal(device.State, result.Data.State)
	s.Require().NotEmpty(result.Key)
}

func (s *DevicesCacheRepositoryTestSuite) TestSetDevice_AllStates() {
	ctx := context.Background()
	states := []model.State{model.StateAvailable, model.StateInUse, model.StateInactive}

	for _, state := range states {
		device := model.NewDevice("Device", "Brand", state)
		err := s.repo.SetDevice(ctx, device, time.Hour)
		s.Require().NoError(err)

		result, err := s.repo.GetDevice(ctx, device.ID)
		s.Require().NoError(err)
		s.Require().True(result.Hit)
		s.Require().Equal(state, result.Data.State)
	}
}

func (s *DevicesCacheRepositoryTestSuite) TestInvalidateDevice() {
	ctx := context.Background()
	device := model.NewDevice("Test Device", "Test Brand", model.StateAvailable)

	err := s.repo.SetDevice(ctx, device, time.Hour)
	s.Require().NoError(err)

	result, err := s.repo.GetDevice(ctx, device.ID)
	s.Require().NoError(err)
	s.Require().True(result.Hit)

	err = s.repo.InvalidateDevice(ctx, device.ID)
	s.Require().NoError(err)

	result, err = s.repo.GetDevice(ctx, device.ID)
	s.Require().NoError(err)
	s.Require().False(result.Hit)
}

func (s *DevicesCacheRepositoryTestSuite) TestInvalidateDevice_NonExistent() {
	ctx := context.Background()
	id := model.NewDeviceID()

	err := s.repo.InvalidateDevice(ctx, id)
	s.Require().NoError(err)
}

func (s *DevicesCacheRepositoryTestSuite) TestDeviceExpiration() {
	ctx := context.Background()
	device := model.NewDevice("Expiring Device", "Brand", model.StateAvailable)

	err := s.repo.SetDevice(ctx, device, time.Millisecond*100)
	s.Require().NoError(err)

	result, err := s.repo.GetDevice(ctx, device.ID)
	s.Require().NoError(err)
	s.Require().True(result.Hit)

	s.miniRedis.FastForward(time.Millisecond * 200)

	result, err = s.repo.GetDevice(ctx, device.ID)
	s.Require().NoError(err)
	s.Require().False(result.Hit)
}

func (s *DevicesCacheRepositoryTestSuite) TestGetDeviceList_NotCached() {
	ctx := context.Background()
	filter := model.DefaultDeviceFilter()

	result, err := s.repo.GetDeviceList(ctx, filter)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().False(result.Hit)
	s.Require().Nil(result.Data)
}

func (s *DevicesCacheRepositoryTestSuite) TestSetAndGetDeviceList() {
	ctx := context.Background()
	devices := []*model.Device{
		model.NewDevice("Device 1", "Brand A", model.StateAvailable),
		model.NewDevice("Device 2", "Brand B", model.StateInUse),
	}
	list := &model.DeviceList{
		Devices: devices,
		Pagination: model.Pagination{
			Page:       1,
			Size:       20,
			TotalItems: 2,
			TotalPages: 1,
		},
		Filters: model.DefaultDeviceFilter(),
	}
	filter := model.DefaultDeviceFilter()
	ttl := time.Hour

	err := s.repo.SetDeviceList(ctx, list, filter, ttl)
	s.Require().NoError(err)

	result, err := s.repo.GetDeviceList(ctx, filter)

	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().True(result.Hit)
	s.Require().NotNil(result.Data)
	s.Require().Len(result.Data.Devices, 2)
	s.Require().Equal(uint(2), result.Data.Pagination.TotalItems)
	s.Require().NotEmpty(result.Key)
}

func (s *DevicesCacheRepositoryTestSuite) TestDeviceList_DifferentFilters() {
	ctx := context.Background()

	filter1 := model.DeviceFilter{
		Brands: []string{"Apple"},
		Page:   1,
		Size:   10,
		Sort:   []string{"-createdAt"},
	}
	list1 := &model.DeviceList{
		Devices:    []*model.Device{model.NewDevice("iPhone", "Apple", model.StateAvailable)},
		Pagination: model.Pagination{TotalItems: 1},
		Filters:    filter1,
	}

	filter2 := model.DeviceFilter{
		Brands: []string{"Samsung"},
		Page:   1,
		Size:   10,
		Sort:   []string{"-createdAt"},
	}
	list2 := &model.DeviceList{
		Devices:    []*model.Device{model.NewDevice("Galaxy", "Samsung", model.StateAvailable)},
		Pagination: model.Pagination{TotalItems: 1},
		Filters:    filter2,
	}

	err := s.repo.SetDeviceList(ctx, list1, filter1, time.Hour)
	s.Require().NoError(err)

	err = s.repo.SetDeviceList(ctx, list2, filter2, time.Hour)
	s.Require().NoError(err)

	result1, err := s.repo.GetDeviceList(ctx, filter1)
	s.Require().NoError(err)
	s.Require().True(result1.Hit)
	s.Require().Equal("Apple", result1.Data.Devices[0].Brand)

	result2, err := s.repo.GetDeviceList(ctx, filter2)
	s.Require().NoError(err)
	s.Require().True(result2.Hit)
	s.Require().Equal("Samsung", result2.Data.Devices[0].Brand)
}

func (s *DevicesCacheRepositoryTestSuite) TestInvalidateAllLists() {
	ctx := context.Background()

	filter1 := model.DeviceFilter{Brands: []string{"Apple"}, Page: 1, Size: 10}
	filter2 := model.DeviceFilter{States: []model.State{model.StateAvailable}, Page: 1, Size: 20}

	list1 := &model.DeviceList{
		Devices:    []*model.Device{model.NewDevice("iPhone", "Apple", model.StateAvailable)},
		Pagination: model.Pagination{TotalItems: 1},
	}
	list2 := &model.DeviceList{
		Devices:    []*model.Device{model.NewDevice("Galaxy", "Samsung", model.StateAvailable)},
		Pagination: model.Pagination{TotalItems: 1},
	}

	err := s.repo.SetDeviceList(ctx, list1, filter1, time.Hour)
	s.Require().NoError(err)

	err = s.repo.SetDeviceList(ctx, list2, filter2, time.Hour)
	s.Require().NoError(err)

	result1, err := s.repo.GetDeviceList(ctx, filter1)
	s.Require().NoError(err)
	s.Require().True(result1.Hit)

	result2, err := s.repo.GetDeviceList(ctx, filter2)
	s.Require().NoError(err)
	s.Require().True(result2.Hit)

	err = s.repo.InvalidateAllLists(ctx)
	s.Require().NoError(err)

	result1, err = s.repo.GetDeviceList(ctx, filter1)
	s.Require().NoError(err)
	s.Require().False(result1.Hit)

	result2, err = s.repo.GetDeviceList(ctx, filter2)
	s.Require().NoError(err)
	s.Require().False(result2.Hit)
}

func (s *DevicesCacheRepositoryTestSuite) TestInvalidateAllLists_PreservesDeviceCache() {
	ctx := context.Background()

	device := model.NewDevice("Test Device", "Brand", model.StateAvailable)
	err := s.repo.SetDevice(ctx, device, time.Hour)
	s.Require().NoError(err)

	filter := model.DefaultDeviceFilter()
	list := &model.DeviceList{
		Devices:    []*model.Device{device},
		Pagination: model.Pagination{TotalItems: 1},
	}
	err = s.repo.SetDeviceList(ctx, list, filter, time.Hour)
	s.Require().NoError(err)

	err = s.repo.InvalidateAllLists(ctx)
	s.Require().NoError(err)

	listResult, err := s.repo.GetDeviceList(ctx, filter)
	s.Require().NoError(err)
	s.Require().False(listResult.Hit)

	deviceResult, err := s.repo.GetDevice(ctx, device.ID)
	s.Require().NoError(err)
	s.Require().True(deviceResult.Hit)
}

func (s *DevicesCacheRepositoryTestSuite) TestPurgeAll() {
	ctx := context.Background()

	device := model.NewDevice("Test Device", "Brand", model.StateAvailable)
	err := s.repo.SetDevice(ctx, device, time.Hour)
	s.Require().NoError(err)

	filter := model.DefaultDeviceFilter()
	list := &model.DeviceList{
		Devices:    []*model.Device{device},
		Pagination: model.Pagination{TotalItems: 1},
	}
	err = s.repo.SetDeviceList(ctx, list, filter, time.Hour)
	s.Require().NoError(err)

	err = s.repo.PurgeAll(ctx)
	s.Require().NoError(err)

	deviceResult, err := s.repo.GetDevice(ctx, device.ID)
	s.Require().NoError(err)
	s.Require().False(deviceResult.Hit)

	listResult, err := s.repo.GetDeviceList(ctx, filter)
	s.Require().NoError(err)
	s.Require().False(listResult.Hit)
}

func (s *DevicesCacheRepositoryTestSuite) TestPurgeByPattern() {
	ctx := context.Background()

	device1 := model.NewDevice("Device 1", "Brand", model.StateAvailable)
	device2 := model.NewDevice("Device 2", "Brand", model.StateInUse)

	err := s.repo.SetDevice(ctx, device1, time.Hour)
	s.Require().NoError(err)

	err = s.repo.SetDevice(ctx, device2, time.Hour)
	s.Require().NoError(err)

	filter := model.DefaultDeviceFilter()
	list := &model.DeviceList{
		Devices:    []*model.Device{device1, device2},
		Pagination: model.Pagination{TotalItems: 2},
	}
	err = s.repo.SetDeviceList(ctx, list, filter, time.Hour)
	s.Require().NoError(err)

	count, err := s.repo.PurgeByPattern(ctx, "devices:list:*")
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(count, int64(1))

	result1, err := s.repo.GetDevice(ctx, device1.ID)
	s.Require().NoError(err)
	s.Require().True(result1.Hit)

	listResult, err := s.repo.GetDeviceList(ctx, filter)
	s.Require().NoError(err)
	s.Require().False(listResult.Hit)
}

func (s *DevicesCacheRepositoryTestSuite) TestIsHealthy() {
	ctx := context.Background()

	healthy := s.repo.IsHealthy(ctx)
	s.Require().True(healthy)
}

func (s *DevicesCacheRepositoryTestSuite) TestIsHealthy_AfterClose() {
	ctx := context.Background()

	err := s.keydbClient.Close()
	s.Require().NoError(err)

	healthy := s.repo.IsHealthy(ctx)
	s.Require().False(healthy)
	s.keydbClient = nil
}

func (s *DevicesCacheRepositoryTestSuite) TestDeviceListExpiration() {
	ctx := context.Background()
	filter := model.DefaultDeviceFilter()
	list := &model.DeviceList{
		Devices:    []*model.Device{model.NewDevice("Device", "Brand", model.StateAvailable)},
		Pagination: model.Pagination{TotalItems: 1},
	}

	err := s.repo.SetDeviceList(ctx, list, filter, time.Millisecond*100)
	s.Require().NoError(err)

	result, err := s.repo.GetDeviceList(ctx, filter)
	s.Require().NoError(err)
	s.Require().True(result.Hit)

	s.miniRedis.FastForward(time.Millisecond * 200)

	result, err = s.repo.GetDeviceList(ctx, filter)
	s.Require().NoError(err)
	s.Require().False(result.Hit)
}

func (s *DevicesCacheRepositoryTestSuite) TestCacheKeyConsistency() {
	ctx := context.Background()

	filter := model.DeviceFilter{
		Brands: []string{"Apple", "Samsung"},
		States: []model.State{model.StateAvailable},
		Page:   1,
		Size:   20,
		Sort:   []string{"-createdAt"},
	}

	list := &model.DeviceList{
		Devices:    []*model.Device{model.NewDevice("Device", "Apple", model.StateAvailable)},
		Pagination: model.Pagination{TotalItems: 1},
	}

	err := s.repo.SetDeviceList(ctx, list, filter, time.Hour)
	s.Require().NoError(err)

	result, err := s.repo.GetDeviceList(ctx, filter)
	s.Require().NoError(err)
	s.Require().True(result.Hit)

	sameFilterDifferentOrder := model.DeviceFilter{
		Brands: []string{"Samsung", "Apple"},
		States: []model.State{model.StateAvailable},
		Page:   1,
		Size:   20,
		Sort:   []string{"-createdAt"},
	}

	result, err = s.repo.GetDeviceList(ctx, sameFilterDifferentOrder)
	s.Require().NoError(err)
	s.Require().True(result.Hit, "Cache should hit for same filter with different array order")
}
