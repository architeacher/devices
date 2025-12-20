package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	t.Setenv("APP_ENVIRONMENT", "sandbox")
	t.Setenv("APP_SERVICE_VERSION", "1.0.0")
	t.Setenv("APP_COMMIT_SHA", "1234xwz")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("AUTH_SECRET_KEY", "test-secret-key")
	t.Setenv("DEVICES_GRPC_ADDRESS", "localhost:9090")

	cfg, err := Init()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	assert.Equal(t, "sandbox", cfg.App.Env)
	assert.Equal(t, "api-gateway", cfg.App.ServiceName)
	assert.Equal(t, "1.0.0", cfg.App.ServiceVersion)
	assert.Equal(t, "1234xwz", cfg.App.CommitSHA)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "test-secret-key", cfg.Auth.SecretKey)
	assert.Equal(t, "localhost:9090", cfg.Devices.Address)
}

func TestInit_DefaultValues(t *testing.T) {
	cfg, err := Init()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// App defaults
	assert.Equal(t, "api-gateway", cfg.App.ServiceName)
	assert.Equal(t, "v1", cfg.App.APIVersion)

	// HTTPServer defaults
	assert.Equal(t, "0.0.0.0", cfg.HTTPServer.Host)
	assert.Equal(t, uint(8088), cfg.HTTPServer.Port)

	// Auth defaults
	assert.True(t, cfg.Auth.Enabled)
	assert.Contains(t, cfg.Auth.ValidIssuers, "api-gateway")
	assert.Contains(t, cfg.Auth.ValidIssuers, "auth-service")

	// Vault defaults
	assert.True(t, cfg.SecretStorage.Enabled)
	assert.Equal(t, "http://vault:8200", cfg.SecretStorage.Address)
	assert.Equal(t, "token", cfg.SecretStorage.AuthMethod)
	assert.Equal(t, "api-gateway", cfg.SecretStorage.MountPath)
}

func TestGetEnvironment(t *testing.T) {
	cases := []struct {
		name     string
		env      string
		expected int
	}{
		{
			name:     "production",
			env:      "production",
			expected: Production,
		},
		{
			name:     "prod shorthand",
			env:      "prod",
			expected: Production,
		},
		{
			name:     "staging",
			env:      "staging",
			expected: Staging,
		},
		{
			name:     "stg shorthand",
			env:      "stg",
			expected: Staging,
		},
		{
			name:     "sandbox",
			env:      "sandbox",
			expected: Sandbox,
		},
		{
			name:     "sbx shorthand",
			env:      "sbx",
			expected: Sandbox,
		},
		{
			name:     "development default",
			env:      "development",
			expected: Development,
		},
		{
			name:     "unknown defaults to development",
			env:      "unknown",
			expected: Development,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &ServiceConfig{
				App: AppConfig{Env: tc.env},
			}

			assert.Equal(t, tc.expected, cfg.GetEnvironment())
		})
	}
}

func TestIsProduction(t *testing.T) {
	cases := []struct {
		name     string
		env      string
		expected bool
	}{
		{
			name:     "production returns true",
			env:      "production",
			expected: true,
		},
		{
			name:     "prod returns true",
			env:      "prod",
			expected: true,
		},
		{
			name:     "staging returns false",
			env:      "staging",
			expected: false,
		},
		{
			name:     "development returns false",
			env:      "development",
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &ServiceConfig{
				App: AppConfig{Env: tc.env},
			}

			assert.Equal(t, tc.expected, cfg.IsProduction())
		})
	}
}
