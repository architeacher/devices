package runtime

import "os"

type ServiceOption func(*ServiceCtx)

func WithServiceTermination(ch chan os.Signal) ServiceOption {
	return func(s *ServiceCtx) {
		s.shutdownChannel = ch
	}
}

func WithWaitingForServer() ServiceOption {
	return func(s *ServiceCtx) {
		s.serverReady = make(chan struct{})
	}
}
