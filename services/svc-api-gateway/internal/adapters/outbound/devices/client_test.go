package devices

import (
	"testing"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		cfg       config.DevicesConfig
		backoff   config.BackoffConfig
		wantErr   bool
		errSubstr string
	}{
		{
			name: "creates client with valid config",
			cfg: config.DevicesConfig{
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
			backoff: config.BackoffConfig{
				BaseDelay:  1 * time.Second,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "creates client with circuit breaker enabled",
			cfg: config.DevicesConfig{
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
			backoff: config.BackoffConfig{
				BaseDelay:  1 * time.Second,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "creates client with TLS enabled but no CA file",
			cfg: config.DevicesConfig{
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
			backoff: config.BackoffConfig{
				BaseDelay:  1 * time.Second,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "fails with non-existent CA file",
			cfg: config.DevicesConfig{
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
			backoff: config.BackoffConfig{
				BaseDelay:  1 * time.Second,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   10 * time.Second,
			},
			wantErr:   true,
			errSubstr: "loading TLS credentials",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, err := NewClient(tc.cfg, tc.backoff)

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

func TestClientClose(t *testing.T) {
	t.Parallel()

	cfg := config.DevicesConfig{
		Address:    "localhost:9090",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		CircuitBreaker: config.CircuitBreakerConfig{
			Enabled: false,
		},
		TLS: config.TLSConfig{
			Enabled: false,
		},
	}
	backoff := config.BackoffConfig{
		BaseDelay:  1 * time.Second,
		Multiplier: 1.6,
		Jitter:     0.2,
		MaxDelay:   10 * time.Second,
	}

	client, err := NewClient(cfg, backoff)
	require.NoError(t, err)
	require.NotNil(t, client)

	err = client.Close()
	require.NoError(t, err)

	err = client.Close()
	require.NoError(t, err)
}
