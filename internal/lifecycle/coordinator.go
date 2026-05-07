package lifecycle

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Component interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type Coordinator struct {
	components []Component
	logger     *slog.Logger
	timeout    time.Duration
}

func NewCoordinator(logger *slog.Logger, timeout time.Duration) *Coordinator {
	return &Coordinator{
		logger:  logger,
		timeout: timeout,
	}
}

func (c *Coordinator) Add(comp Component) {
	c.components = append(c.components, comp)
}

func (c *Coordinator) StartAll(ctx context.Context) error {
	for i, comp := range c.components {
		c.logger.Info("starting component", "index", i)
		if err := comp.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *Coordinator) StopAll(ctx context.Context) {
	shutdownCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	for i := len(c.components) - 1; i >= 0; i-- {
		c.logger.Info("stopping component", "index", i)
		if err := c.components[i].Stop(shutdownCtx); err != nil {
			c.logger.Error("failed to stop component", "index", i, "error", err)
		}
	}
}

func (c *Coordinator) WaitForSignal(ctx context.Context) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-ctx.Done():
	case <-sigCh:
	}
}
