package runtime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
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
	c.monitorConfigChanges()

	// Waits for one of the following shutdown conditions to happen.
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

		addr := fmt.Sprintf(":%d", c.deps.config.PublicHTTPServer.Port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("failed to listen on %s: %v", addr, err)
		}

		c.deps.infra.logger.Info().
			Str("address", addr).
			Msg("starting the http server")

		if c.serverReady != nil {
			close(c.serverReady)
		}

		if err := c.deps.infra.publicHttpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("PublicHTTPServer server error: %v", err)
		}
	}()

	c.startAdminServer()
}

func (c *ServiceCtx) startAdminServer() {
	if c.deps.infra.adminHttpServer == nil {
		return
	}

	go func() {
		cfg := c.deps.config.AdminHTTPServer
		addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))

		listener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("failed to listen on admin server %s: %v", addr, err)
		}

		c.deps.infra.logger.Info().
			Str("address", addr).
			Msg("starting the admin http server")

		if err := c.deps.infra.adminHttpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("admin PublicHTTPServer error: %v", err)
		}
	}()
}

func (c *ServiceCtx) monitorConfigChanges() {
	if c.deps.configLoader == nil {
		return
	}

	reloadErrors := c.deps.configLoader.WatchConfigSignals(c.serverCtx)
	go func() {
		for err := range reloadErrors {
			if err != nil {
				c.deps.infra.logger.Error().Err(err).Msg("config reload failed")
			} else {
				c.deps.infra.logger.Info().Msg("config reloaded successfully")
			}
		}
	}()
}

func (c *ServiceCtx) shutdownHook() {
	signal.Notify(c.shutdownChannel, syscall.SIGINT, syscall.SIGTERM)
}

func (c *ServiceCtx) shutdown() {
	c.deps.infra.logger.Info().Msg("shutting down service...")

	// Cancel context that underlying processes would start cleanup.
	c.serverStopFunc()

	// Shutdown signal with a grace period of 30 seconds.
	shutdownCtx, cancel := context.WithTimeout(c.serverCtx, c.deps.config.PublicHTTPServer.ShutdownTimeout)

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

// WaitForServer blocks until the http server is running.
// If you want to be notified when the server is running,
// make sure you instantiate your server with WithWaitingForServer.
//
// Example:
//
//	srv := runtime.New(WithWaitingForServer())
//	go func() {
//		srv.Run()
//	}()
//
//	srv.WaitForServer()
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
