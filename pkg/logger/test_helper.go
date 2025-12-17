package logger

import (
	"io"

	"github.com/rs/zerolog"
)

// NewTestLogger returns a Logger backed by a nop zerolog instance.
func NewTestLogger() Logger {
	return Logger{Logger: zerolog.Nop()}
}

func NewBufferedTestLogger(w io.Writer) Logger {
	return Logger{Logger: zerolog.New(w)}
}
