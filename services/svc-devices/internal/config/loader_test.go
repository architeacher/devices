package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	t.Setenv("APP_ENVIRONMENT", "sandbox")
	t.Setenv("APP_SERVICE_NAME", "svc-devices")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Init()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	assert.Equal(t, "sandbox", cfg.App.Env.Name)
	assert.Equal(t, "svc-devices", cfg.App.ServiceName)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestInit_DefaultValues(t *testing.T) {
	cfg, err := Init()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// App defaults
	assert.Equal(t, "svc-devices", cfg.App.ServiceName)
	assert.Equal(t, "v1", cfg.App.APIVersion)

	// HTTPServer defaults
	assert.Equal(t, "0.0.0.0", cfg.GRPCServer.Host)
	assert.Equal(t, uint(9090), cfg.GRPCServer.Port)

	// Vault defaults
	assert.True(t, cfg.SecretsStorage.Enabled)
	assert.Equal(t, "http://vault:8200", cfg.SecretsStorage.Address)
	assert.Equal(t, "token", cfg.SecretsStorage.AuthMethod)
	assert.Equal(t, "svc-devices", cfg.SecretsStorage.MountPath)
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
				App: App{Env: Environment{Name: tc.env}},
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
				App: App{Env: Environment{Name: tc.env}},
			}

			assert.Equal(t, tc.expected, cfg.IsProduction())
		})
	}
}
