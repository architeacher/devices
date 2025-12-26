package services

import (
	"testing"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/stretchr/testify/require"
)

func TestNewDevicesService(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		cfg       *config.ServiceConfig
		wantErr   bool
		errSubstr string
	}{
		{
			name: "creates client with valid config",
			cfg: &config.ServiceConfig{
				DevicesGRPCClient: config.DevicesGRPCClient{
					Address:    "localhost:9090",
					Timeout:    30 * time.Second,
					MaxRetries: 3,
					CircuitBreaker: config.CircuitBreakerConfig{
						Enabled:          false,
						MaxRequests:      5,
						Interval:         60 * time.Second,
						Timeout:          30 * time.Second,
						FailureThreshold: 5,
					},
					TLS: config.TLSConfig{
						Enabled: false,
					},
				},
				Backoff: config.Backoff{
					BaseDelay:  1 * time.Second,
					Multiplier: 1.6,
					Jitter:     0.2,
					MaxDelay:   10 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "creates client with circuit breaker enabled",
			cfg: &config.ServiceConfig{
				DevicesGRPCClient: config.DevicesGRPCClient{
					Address:    "localhost:9090",
					Timeout:    30 * time.Second,
					MaxRetries: 3,
					CircuitBreaker: config.CircuitBreakerConfig{
						Enabled:          true,
						MaxRequests:      5,
						Interval:         60 * time.Second,
						Timeout:          30 * time.Second,
						FailureThreshold: 5,
					},
					TLS: config.TLSConfig{
						Enabled: false,
					},
				},
				Backoff: config.Backoff{
					BaseDelay:  1 * time.Second,
					Multiplier: 1.6,
					Jitter:     0.2,
					MaxDelay:   10 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "creates client with TLS enabled but no CA file",
			cfg: &config.ServiceConfig{
				DevicesGRPCClient: config.DevicesGRPCClient{
					Address:    "localhost:9090",
					Timeout:    30 * time.Second,
					MaxRetries: 3,
					CircuitBreaker: config.CircuitBreakerConfig{
						Enabled: false,
					},
					TLS: config.TLSConfig{
						Enabled: true,
					},
				},
				Backoff: config.Backoff{
					BaseDelay:  1 * time.Second,
					Multiplier: 1.6,
					Jitter:     0.2,
					MaxDelay:   10 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "fails with non-existent CA file",
			cfg: &config.ServiceConfig{
				DevicesGRPCClient: config.DevicesGRPCClient{
					Address:    "localhost:9090",
					Timeout:    30 * time.Second,
					MaxRetries: 3,
					CircuitBreaker: config.CircuitBreakerConfig{
						Enabled: false,
					},
					TLS: config.TLSConfig{
						Enabled: true,
						CAFile:  "/non/existent/ca.pem",
					},
				},
				Backoff: config.Backoff{
					BaseDelay:  1 * time.Second,
					Multiplier: 1.6,
					Jitter:     0.2,
					MaxDelay:   10 * time.Second,
				},
			},
			wantErr:   true,
			errSubstr: "loading TLS credentials",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, err := NewDevicesService(tc.cfg)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errSubstr)
				require.Nil(t, client)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
			require.NoError(t, client.Close())
		})
	}
}

func TestDevicesServiceClose(t *testing.T) {
	t.Parallel()

	cfg := &config.ServiceConfig{
		DevicesGRPCClient: config.DevicesGRPCClient{
			Address:    "localhost:9090",
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			CircuitBreaker: config.CircuitBreakerConfig{
				Enabled: false,
			},
			TLS: config.TLSConfig{
				Enabled: false,
			},
		},
		Backoff: config.Backoff{
			BaseDelay:  1 * time.Second,
			Multiplier: 1.6,
			Jitter:     0.2,
			MaxDelay:   10 * time.Second,
		},
	}

	client, err := NewDevicesService(cfg)
	require.NoError(t, err)
	require.NotNil(t, client)

	err = client.Close()
	require.NoError(t, err)

	err = client.Close()
	require.NoError(t, err)
}
