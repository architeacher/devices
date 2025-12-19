package runtime

type ServiceOption func(*ServiceCtx)

func WithWaitingForServer() ServiceOption {
	return func(c *ServiceCtx) {
		c.serverReady = make(chan struct{}, 1)
	}
}
