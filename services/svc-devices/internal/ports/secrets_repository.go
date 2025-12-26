//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

package ports

import (
	"context"

	"github.com/hashicorp/vault/api"
)

//counterfeiter:generate -o ../mocks/secrets_repository.go . SecretsRepository

type (
	// SecretsRepository defines the interface for interacting with a secrets storage backend.
	SecretsRepository interface {
		// SetToken sets the authentication token for the secrets' repository.
		SetToken(v string)
		// GetSecrets retrieves secrets from the specified path.
		GetSecrets(ctx context.Context, path string) (*api.Secret, error)
		// WriteWithContext writes data to the specified path.
		WriteWithContext(ctx context.Context, path string, data map[string]any) (*api.Secret, error)
	}
)
