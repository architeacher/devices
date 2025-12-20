package logger

import (
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const (
	JSONLoggingFormat = "json"

	LogLevelDebug   = "debug"
	LogLevelInfo    = "info"
	LogLevelWarn    = "warn"
	LogLevelWarning = "warning"
	LogLevelError   = "error"
	LogLevelFatal   = "fatal"
	LogLevelPanic   = "panic"

	ContextKeyRequestID     contextKey = "requestID"
	ContextKeyCorrelationID contextKey = "correlationID"
)

type Logger struct {
	zerolog.Logger
}

func New(level, format string) Logger {
	return NewWithWriter(level, format, os.Stdout)
}

func NewWithWriter(level, format string, w io.Writer) Logger {
	var logLevel zerolog.Level

	switch strings.ToLower(level) {
	case LogLevelDebug:
		logLevel = zerolog.DebugLevel
	case LogLevelInfo:
		logLevel = zerolog.InfoLevel
	case LogLevelWarn, LogLevelWarning:
		logLevel = zerolog.WarnLevel
	case LogLevelError:
		logLevel = zerolog.ErrorLevel
	case LogLevelFatal:
		logLevel = zerolog.FatalLevel
	case LogLevelPanic:
		logLevel = zerolog.PanicLevel
	default:
		logLevel = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(logLevel)

	logger := zerolog.New(zerolog.ConsoleWriter{Out: w, TimeFormat: time.RFC3339})

	if format == JSONLoggingFormat {
		logger = zerolog.New(w)
	}

	logger = logger.With().Timestamp().Logger()

	return Logger{
		Logger: logger,
	}
}

func (l Logger) WithContext(ctx context.Context) zerolog.Logger {
	logger := l.Logger

	if correlationID, ok := ctx.Value(ContextKeyCorrelationID).(string); ok && correlationID != "" {
		logger = logger.With().Str("correlation_id", correlationID).Logger()
	}

	if requestID, ok := ctx.Value(ContextKeyRequestID).(string); ok && requestID != "" {
		logger = logger.With().Str("request_id", requestID).Logger()
	}

	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		logger = logger.With().
			Str("trace_id", span.SpanContext().TraceID().String()).
			Str("span_id", span.SpanContext().SpanID().String()).
			Logger()
	}

	return logger
}

