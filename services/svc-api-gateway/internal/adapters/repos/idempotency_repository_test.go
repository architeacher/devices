package repos_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/architeacher/devices/pkg/logger"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/repos"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/infrastructure"
	"github.com/architeacher/devices/services/svc-api-gateway/internal/ports"
	"github.com/stretchr/testify/suite"
)

type IdempotencyRepositoryTestSuite struct {
	suite.Suite
	miniRedis   *miniredis.Miniredis
	keydbClient *infrastructure.KeydbClient
	repo        *repos.IdempotencyRepository
}

func TestIdempotencyRepositoryTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(IdempotencyRepositoryTestSuite))
}

func (s *IdempotencyRepositoryTestSuite) SetupTest() {
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
	s.repo, err = repos.NewIdempotencyRepository(s.keydbClient)
	s.Require().NoError(err)
}

func (s *IdempotencyRepositoryTestSuite) TearDownTest() {
	if s.keydbClient != nil {
		s.keydbClient.Close()
	}
	if s.miniRedis != nil {
		s.miniRedis.Close()
	}
}

func (s *IdempotencyRepositoryTestSuite) TestNewIdempotencyRepository() {
	s.Require().NotNil(s.repo)
}

func (s *IdempotencyRepositoryTestSuite) TestGetNonExistentKey() {
	ctx := context.Background()

	response, err := s.repo.Get(ctx, "non-existent-key")
	s.Require().NoError(err)
	s.Require().Nil(response)
}

func (s *IdempotencyRepositoryTestSuite) TestSetAndGet() {
	ctx := context.Background()
	key := "test-key"
	cachedResponse := &ports.CachedResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"id": "123"}`),
		CreatedAt:  time.Now().UTC(),
	}

	err := s.repo.Set(ctx, key, cachedResponse, time.Hour)
	s.Require().NoError(err)

	retrieved, err := s.repo.Get(ctx, key)
	s.Require().NoError(err)
	s.Require().NotNil(retrieved)
	s.Require().Equal(cachedResponse.StatusCode, retrieved.StatusCode)
	s.Require().Equal(cachedResponse.Headers, retrieved.Headers)
	s.Require().Equal(cachedResponse.Body, retrieved.Body)
}

func (s *IdempotencyRepositoryTestSuite) TestSetLock() {
	ctx := context.Background()
	key := "lock-test-key"

	acquired, err := s.repo.SetLock(ctx, key, time.Minute)
	s.Require().NoError(err)
	s.Require().True(acquired)

	acquired, err = s.repo.SetLock(ctx, key, time.Minute)
	s.Require().NoError(err)
	s.Require().False(acquired)
}

func (s *IdempotencyRepositoryTestSuite) TestReleaseLock() {
	ctx := context.Background()
	key := "release-lock-test-key"

	acquired, err := s.repo.SetLock(ctx, key, time.Minute)
	s.Require().NoError(err)
	s.Require().True(acquired)

	err = s.repo.ReleaseLock(ctx, key)
	s.Require().NoError(err)

	acquired, err = s.repo.SetLock(ctx, key, time.Minute)
	s.Require().NoError(err)
	s.Require().True(acquired)
}

func (s *IdempotencyRepositoryTestSuite) TestIsHealthy() {
	ctx := context.Background()

	healthy := s.repo.IsHealthy(ctx)
	s.Require().True(healthy)
}

func (s *IdempotencyRepositoryTestSuite) TestIsHealthyAfterClose() {
	ctx := context.Background()

	err := s.keydbClient.Close()
	s.Require().NoError(err)

	healthy := s.repo.IsHealthy(ctx)
	s.Require().False(healthy)
	s.keydbClient = nil
}

func (s *IdempotencyRepositoryTestSuite) TestExpiration() {
	ctx := context.Background()
	key := "expiring-key"
	cachedResponse := &ports.CachedResponse{
		StatusCode: 200,
		Headers:    map[string]string{},
		Body:       []byte(`{}`),
		CreatedAt:  time.Now().UTC(),
	}

	err := s.repo.Set(ctx, key, cachedResponse, time.Millisecond*100)
	s.Require().NoError(err)

	s.miniRedis.FastForward(time.Millisecond * 200)

	retrieved, err := s.repo.Get(ctx, key)
	s.Require().NoError(err)
	s.Require().Nil(retrieved)
}
