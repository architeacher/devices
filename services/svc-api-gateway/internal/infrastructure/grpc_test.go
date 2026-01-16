package infrastructure

import (
	"testing"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
	"github.com/stretchr/testify/require"
)

func testConfig() *config.ServiceConfig {
	return &config.ServiceConfig{
		DevicesGRPCClient: config.DevicesGRPCClient{
			Address:        "localhost:9090",
			Timeout:        30 * time.Second,
			MaxRetries:     3,
			MaxMessageSize: 4194304,
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
}

func TestNewGRPCConnection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		cfg       *config.ServiceConfig
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "creates connection with valid config",
			cfg:     testConfig(),
			wantErr: false,
		},
		{
			name: "creates connection with TLS enabled but no CA file",
			cfg: func() *config.ServiceConfig {
				cfg := testConfig()
				cfg.DevicesGRPCClient.TLS.Enabled = true

				return cfg
			}(),
			wantErr: false,
		},
		{
			name: "fails with non-existent CA file",
			cfg: func() *config.ServiceConfig {
				cfg := testConfig()
				cfg.DevicesGRPCClient.TLS.Enabled = true
				cfg.DevicesGRPCClient.TLS.CAFile = "/non/existent/ca.pem"

				return cfg
			}(),
			wantErr:   true,
			errSubstr: "loading TLS credentials",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			conn, err := NewGRPCConnection(tc.cfg)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errSubstr)
				require.Nil(t, conn)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, conn)
			require.NoError(t, conn.Close())
		})
	}
}

func TestNewGRPCConnection_Close(t *testing.T) {
	t.Parallel()

	conn, err := NewGRPCConnection(testConfig())
	require.NoError(t, err)
	require.NotNil(t, conn)

	err = conn.Close()
	require.NoError(t, err)
}
