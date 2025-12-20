package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/architeacher/devices/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		level  string
		format string
	}{
		{
			name:   "creates logger with debug level",
			level:  logger.LogLevelDebug,
			format: "console",
		},
		{
			name:   "creates logger with info level",
			level:  logger.LogLevelInfo,
			format: "console",
		},
		{
			name:   "creates logger with json format",
			level:  logger.LogLevelInfo,
			format: logger.JSONLoggingFormat,
		},
		{
			name:   "creates logger with default level for unknown",
			level:  "unknown",
			format: "console",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			log := logger.New(tc.level, tc.format)
			require.NotNil(t, log)
		})
	}
}

func TestWithContext(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		setupContext      func() context.Context
		expectedRequestID string
		hasRequestID      bool
	}{
		{
			name: "adds request ID to logger",
			setupContext: func() context.Context {
				return context.WithValue(context.Background(), logger.ContextKeyRequestID, "test-request-123")
			},
			expectedRequestID: "test-request-123",
			hasRequestID:      true,
		},
		{
			name: "handles empty context",
			setupContext: func() context.Context {
				return context.Background()
			},
			hasRequestID: false,
		},
		{
			name: "handles empty request ID",
			setupContext: func() context.Context {
				return context.WithValue(context.Background(), logger.ContextKeyRequestID, "")
			},
			hasRequestID: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			log := logger.NewWithWriter(logger.LogLevelInfo, logger.JSONLoggingFormat, &buf)

			ctx := tc.setupContext()
			ctxLogger := log.WithContext(ctx)

			ctxLogger.Info().Msg("test message")

			if tc.hasRequestID {
				var logEntry map[string]any
				err := json.Unmarshal(buf.Bytes(), &logEntry)
				require.NoError(t, err)
				require.Equal(t, tc.expectedRequestID, logEntry["request_id"])
			}
		})
	}
}

