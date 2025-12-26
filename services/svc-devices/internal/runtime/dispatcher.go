package runtime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

type ServiceCtx struct {
	deps            *dependencies
	shutdownChannel chan os.Signal
	serverCtx       context.Context
	serverStopFunc  context.CancelFunc
	serverReady     chan struct{}
}

func New(opts ...ServiceOption) *ServiceCtx {
	ctx := &ServiceCtx{
		shutdownChannel: make(chan os.Signal, 1),
	}

	for _, opt := range opts {
		opt(ctx)
	}

	return ctx
}

func (c *ServiceCtx) Run() {
	if err := c.build(); err != nil {
		log.Fatalf("failed to build service: %v", err)
	}

	c.startService()
	c.shutdownHook()

	select {
	case <-c.serverCtx.Done():
	case <-c.shutdownChannel:
		defer close(c.shutdownChannel)
	}

	c.shutdown()
}

func (c *ServiceCtx) build() error {
	c.serverCtx, c.serverStopFunc = context.WithCancel(context.Background())

	var err error

	c.deps, err = initializeDependencies(c.serverCtx)
	if err != nil {
		return fmt.Errorf("initializing dependencies: %w", err)
	}

	return nil
}

func (c *ServiceCtx) startService() {
	go func() {
		if c.serverReady != nil {
			c.serverReady <- struct{}{}
		}

		addr := fmt.Sprintf(":%d", c.deps.config.GRPCServer.Port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("failed to listen on %s: %v", addr, err)
		}

		c.deps.infra.logger.Info().
			Str("address", addr).
			Msg("starting the gRPC server")

		if c.serverReady != nil {
			close(c.serverReady)
		}

		if err := c.deps.infra.grpcServer.Serve(listener); err != nil {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()
}

func (c *ServiceCtx) shutdownHook() {
	signal.Notify(c.shutdownChannel, syscall.SIGINT, syscall.SIGTERM)
}

func (c *ServiceCtx) shutdown() {
	c.deps.infra.logger.Info().Msg("shutting down service...")

	c.serverStopFunc()

	shutdownCtx, cancel := context.WithTimeout(c.serverCtx, c.deps.config.GRPCServer.ShutdownTimeout)

	go func() {
		<-shutdownCtx.Done()

		if errors.Is(shutdownCtx.Err(), context.DeadlineExceeded) {
			c.deps.infra.logger.Error().Msg("graceful shutdown timed out.. forcing exit.")
			cancel()
			os.Exit(1)
		}
	}()

	c.cleanup(shutdownCtx)

	c.deps.infra.logger.Info().Msg("service shutdown complete")
}

func (c *ServiceCtx) WaitForServer() {
	if c.serverReady != nil {
		<-c.serverReady
	}
}

func (c *ServiceCtx) cleanup(shutdownCtx context.Context) {
	c.deps.infra.logger.Info().Msg("cleaning up resources...")

	for resource, cleanupFn := range c.deps.cleanupFuncs {
		if err := cleanupFn(shutdownCtx); err != nil {
			c.deps.infra.logger.Error().
				Err(err).
				Str("resource", resource).
				Msg("failed to shutdown the resource gracefully")
		}
	}

	c.deps.infra.logger.Info().Msg("cleanup completed")
}
