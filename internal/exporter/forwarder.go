package exporter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/metrics"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

type Forwarder struct {
	endpoint string
	insecure bool
	logger   *slog.Logger
	client   *http.Client
	m        *metrics.Metrics

	metricsSent atomic.Int64
	tracesSent  atomic.Int64
	logsSent    atomic.Int64
	metricsErr  atomic.Int64
	tracesErr   atomic.Int64
	logsErr     atomic.Int64
}

func New(endpoint string, insecure bool, logger *slog.Logger) *Forwarder {
	return NewWithMetrics(endpoint, insecure, logger, nil)
}

func NewWithMetrics(endpoint string, insecure bool, logger *slog.Logger, m *metrics.Metrics) *Forwarder {
	scheme := "https"
	if insecure {
		scheme = "http"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, endpoint)

	return &Forwarder{
		endpoint: baseURL,
		insecure: insecure,
		logger:   logger,
		m:        m,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (f *Forwarder) ForwardMetrics(ctx context.Context, protoBytes []byte) error {
	return f.withRetry(ctx, "metrics", 3, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.endpoint+"/v1/metrics", bytes.NewReader(protoBytes))
		if err != nil {
			return fmt.Errorf("forwarder: create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-protobuf")

		resp, err := f.client.Do(req)
		if err != nil {
			return fmt.Errorf("forwarder: send metrics: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(io.Discard, resp.Body)

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("forwarder: metrics backend returned status %d", resp.StatusCode)
		}
		f.metricsSent.Add(1)
		if f.m != nil {
			f.m.SignalsForwarded.WithLabelValues("metric", "http").Inc()
		}
		return nil
	})
}

func (f *Forwarder) ForwardTraces(ctx context.Context, protoBytes []byte) error {
	return f.withRetry(ctx, "traces", 3, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.endpoint+"/v1/traces", bytes.NewReader(protoBytes))
		if err != nil {
			return fmt.Errorf("forwarder: create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-protobuf")

		resp, err := f.client.Do(req)
		if err != nil {
			return fmt.Errorf("forwarder: send traces: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(io.Discard, resp.Body)

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("forwarder: traces backend returned status %d", resp.StatusCode)
		}
		f.tracesSent.Add(1)
		if f.m != nil {
			f.m.SignalsForwarded.WithLabelValues("trace", "http").Inc()
		}
		return nil
	})
}

func (f *Forwarder) ForwardLogs(ctx context.Context, protoBytes []byte) error {
	return f.withRetry(ctx, "logs", 3, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.endpoint+"/v1/logs", bytes.NewReader(protoBytes))
		if err != nil {
			return fmt.Errorf("forwarder: create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-protobuf")

		resp, err := f.client.Do(req)
		if err != nil {
			return fmt.Errorf("forwarder: send logs: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(io.Discard, resp.Body)

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("forwarder: logs backend returned status %d", resp.StatusCode)
		}
		f.logsSent.Add(1)
		if f.m != nil {
			f.m.SignalsForwarded.WithLabelValues("log", "http").Inc()
		}
		return nil
	})
}

func (f *Forwarder) ForwardMetricsFromData(ctx context.Context, data []byte) error {
	return f.ForwardMetrics(ctx, data)
}

func (f *Forwarder) ForwardTracesFromData(ctx context.Context, data []byte) error {
	return f.ForwardTraces(ctx, data)
}

func (f *Forwarder) ForwardLogsFromData(ctx context.Context, data []byte) error {
	return f.ForwardLogs(ctx, data)
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
		if f.m != nil {
			f.m.SignalsDropped.WithLabelValues("metric", "http").Inc()
		}
	case "traces":
		f.tracesErr.Add(1)
		if f.m != nil {
			f.m.SignalsDropped.WithLabelValues("trace", "http").Inc()
		}
	case "logs":
		f.logsErr.Add(1)
		if f.m != nil {
			f.m.SignalsDropped.WithLabelValues("log", "http").Inc()
		}
	}
	return fmt.Errorf("exporter: %s: failed after %d retries: %w", signalType, maxRetries, lastErr)
}

func (f *Forwarder) Stats() (metricsSent, tracesSent, logsSent, metricsErr, tracesErr, logsErr int64) {
	return f.metricsSent.Load(), f.tracesSent.Load(), f.logsSent.Load(),
		f.metricsErr.Load(), f.tracesErr.Load(), f.logsErr.Load()
}

func MarshalMetricsExportRequest(protoBytes []byte) ([]byte, error) {
	req := pmetricotlp.NewExportRequest()
	if err := req.UnmarshalProto(protoBytes); err != nil {
		return nil, err
	}
	return req.MarshalProto()
}

func MarshalTracesExportRequest(protoBytes []byte) ([]byte, error) {
	req := ptraceotlp.NewExportRequest()
	if err := req.UnmarshalProto(protoBytes); err != nil {
		return nil, err
	}
	return req.MarshalProto()
}

func MarshalLogsExportRequest(protoBytes []byte) ([]byte, error) {
	req := plogotlp.NewExportRequest()
	if err := req.UnmarshalProto(protoBytes); err != nil {
		return nil, err
	}
	return req.MarshalProto()
}
