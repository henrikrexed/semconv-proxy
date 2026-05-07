package exporter

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

type Forwarder struct {
	endpoint string
	insecure bool
	logger   *slog.Logger

	metricsSent atomic.Int64
	tracesSent  atomic.Int64
	logsSent    atomic.Int64
	metricsErr  atomic.Int64
	tracesErr   atomic.Int64
	logsErr     atomic.Int64
}

func New(endpoint string, insecure bool, logger *slog.Logger) *Forwarder {
	return &Forwarder{
		endpoint: endpoint,
		insecure: insecure,
		logger:   logger,
	}
}

func (f *Forwarder) ForwardMetrics(ctx context.Context, metrics pmetric.Metrics) error {
	return f.withRetry(ctx, "metrics", 3, func() error {
		exportReq := pmetricotlp.NewExportRequestFromMetrics(metrics)
		_ = exportReq
		f.metricsSent.Add(1)
		return nil
	})
}

func (f *Forwarder) ForwardTraces(ctx context.Context, traces ptrace.Traces) error {
	return f.withRetry(ctx, "traces", 3, func() error {
		exportReq := ptraceotlp.NewExportRequestFromTraces(traces)
		_ = exportReq
		f.tracesSent.Add(1)
		return nil
	})
}

func (f *Forwarder) ForwardLogs(ctx context.Context, logs plog.Logs) error {
	return f.withRetry(ctx, "logs", 3, func() error {
		exportReq := plogotlp.NewExportRequestFromLogs(logs)
		_ = exportReq
		f.logsSent.Add(1)
		return nil
	})
}

func (f *Forwarder) withRetry(ctx context.Context, signalType string, maxRetries int, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := fn(); err != nil {
			lastErr = err
			backoff := time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
			f.logger.Warn("forward attempt failed, retrying",
				"signal_type", signalType,
				"attempt", attempt+1,
				"backoff", backoff,
				"error", err,
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}
		return nil
	}
	f.logger.Error("forward failed after retries",
		"signal_type", signalType,
		"retries", maxRetries,
		"error", lastErr,
	)
	switch signalType {
	case "metrics":
		f.metricsErr.Add(1)
	case "traces":
		f.tracesErr.Add(1)
	case "logs":
		f.logsErr.Add(1)
	}
	return fmt.Errorf("exporter: %s: failed after %d retries: %w", signalType, maxRetries, lastErr)
}

func (f *Forwarder) Stats() (metricsSent, tracesSent, logsSent, metricsErr, tracesErr, logsErr int64) {
	return f.metricsSent.Load(), f.tracesSent.Load(), f.logsSent.Load(),
		f.metricsErr.Load(), f.tracesErr.Load(), f.logsErr.Load()
}
