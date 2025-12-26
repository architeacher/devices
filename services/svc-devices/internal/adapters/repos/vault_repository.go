package repos

import (
	"context"

	"github.com/hashicorp/vault/api"
)

type (
	VaultRepository struct {
		vaultClient *api.Client
	}
)

func NewVaultRepository(vaultClient *api.Client) *VaultRepository {
	return &VaultRepository{
		vaultClient: vaultClient,
	}
}

func (r *VaultRepository) SetToken(v string) {
	r.vaultClient.SetToken(v)
}

func (r *VaultRepository) GetSecrets(ctx context.Context, path string) (*api.Secret, error) {
	return r.vaultClient.Logical().ReadWithContext(ctx, path)
}

func (r *VaultRepository) WriteWithContext(ctx context.Context, path string, data map[string]any) (*api.Secret, error) {
	return r.vaultClient.Logical().WriteWithContext(ctx, path, data)
}
