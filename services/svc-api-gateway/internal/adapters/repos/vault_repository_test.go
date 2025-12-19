package repos_test

import (
	"testing"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/adapters/repos"
	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/suite"
)

type VaultRepositoryTestSuite struct {
	suite.Suite
}

func TestVaultRepositoryTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(VaultRepositoryTestSuite))
}

func (s *VaultRepositoryTestSuite) TestNewVaultRepository() {
	s.T().Parallel()

	client, err := api.NewClient(api.DefaultConfig())
	s.Require().NoError(err)

	repo := repos.NewVaultRepository(client)
	s.Require().NotNil(repo)
}

func (s *VaultRepositoryTestSuite) TestSetToken() {
	s.T().Parallel()

	client, err := api.NewClient(api.DefaultConfig())
	s.Require().NoError(err)

	repo := repos.NewVaultRepository(client)
	s.Require().NotNil(repo)

	// Should not panic
	repo.SetToken("test-token")
}
