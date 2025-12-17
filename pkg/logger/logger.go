package logger

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

const (
	JSONLoggingFormat = "json"

	LogLevelDebug   = "debug"
	LogLevelInfo    = "info"
	LogLevelWarn    = "warn"
	LogLevelWarning = "warning"
	LogLevelError   = "error"
	LogLevelFatal   = "fatal"
	LogLevelPanic   = "panic"
)

type Logger struct {
	zerolog.Logger
}

func New(level, format string) Logger {
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

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	if format == JSONLoggingFormat {
		logger = zerolog.New(os.Stdout)
	}

	logger = logger.With().Timestamp().Logger()

	return Logger{
		Logger: logger,
	}
}
