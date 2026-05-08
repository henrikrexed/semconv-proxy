package exporter

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"context"
	"log/slog"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

func TestForwardMetricsToBackend(t *testing.T) {
	received := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		if r.URL.Path != "/v1/metrics" {
			t.Errorf("path = %q, want /v1/metrics", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/x-protobuf" {
			t.Errorf("Content-Type = %q, want application/x-protobuf", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
		resp, _ := pmetricotlp.NewExportResponse().MarshalProto()
		_, _ = w.Write(resp)
	}))
	defer ts.Close()

	fwd := New(ts.URL[len("http://"):], true, slog.Default())

	metrics := pmetric.NewMetrics()
	protoBytes, _ := pmetricotlp.NewExportRequestFromMetrics(metrics).MarshalProto()

	err := fwd.ForwardMetrics(context.Background(), protoBytes)
	if err != nil {
		t.Fatalf("ForwardMetrics error: %v", err)
	}
	if !received {
		t.Error("expected backend to receive request")
	}
}

func TestForwardTracesToBackend(t *testing.T) {
	received := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
		resp, _ := ptraceotlp.NewExportResponse().MarshalProto()
		_, _ = w.Write(resp)
	}))
	defer ts.Close()

	fwd := New(ts.URL[len("http://"):], true, slog.Default())

	traces := ptrace.NewTraces()
	protoBytes, _ := ptraceotlp.NewExportRequestFromTraces(traces).MarshalProto()

	err := fwd.ForwardTraces(context.Background(), protoBytes)
	if err != nil {
		t.Fatalf("ForwardTraces error: %v", err)
	}
	if !received {
		t.Error("expected backend to receive request")
	}
}

func TestForwardLogsToBackend(t *testing.T) {
	received := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
		resp, _ := plogotlp.NewExportResponse().MarshalProto()
		_, _ = w.Write(resp)
	}))
	defer ts.Close()

	fwd := New(ts.URL[len("http://"):], true, slog.Default())

	logs := plog.NewLogs()
	protoBytes, _ := plogotlp.NewExportRequestFromLogs(logs).MarshalProto()

	err := fwd.ForwardLogs(context.Background(), protoBytes)
	if err != nil {
		t.Fatalf("ForwardLogs error: %v", err)
	}
	if !received {
		t.Error("expected backend to receive request")
	}
}

func TestForwardRetryOnFailure(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		resp, _ := pmetricotlp.NewExportResponse().MarshalProto()
		_, _ = w.Write(resp)
	}))
	defer ts.Close()

	fwd := New(ts.URL[len("http://"):], true, slog.Default())

	metrics := pmetric.NewMetrics()
	protoBytes, _ := pmetricotlp.NewExportRequestFromMetrics(metrics).MarshalProto()

	err := fwd.ForwardMetrics(context.Background(), protoBytes)
	if err != nil {
		t.Fatalf("ForwardMetrics error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestForwardStats(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp, _ := pmetricotlp.NewExportResponse().MarshalProto()
		_, _ = w.Write(resp)
	}))
	defer ts.Close()

	fwd := New(ts.URL[len("http://"):], true, slog.Default())

	metrics := pmetric.NewMetrics()
	protoBytes, _ := pmetricotlp.NewExportRequestFromMetrics(metrics).MarshalProto()

	_ = fwd.ForwardMetrics(context.Background(), protoBytes)

	ms, ts2, ls, me, te, le := fwd.Stats()
	if ms != 1 {
		t.Errorf("metricsSent = %d, want 1", ms)
	}
	if me != 0 {
		t.Errorf("metricsErr = %d, want 0", me)
	}
	_ = ts2
	_ = ls
	_ = te
	_ = le
}

func TestForwardAllRetriesExhausted(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	fwd := New(ts.URL[len("http://"):], true, slog.Default())

	metrics := pmetric.NewMetrics()
	protoBytes, _ := pmetricotlp.NewExportRequestFromMetrics(metrics).MarshalProto()

	err := fwd.ForwardMetrics(context.Background(), protoBytes)
	if err == nil {
		t.Fatal("expected error when all retries exhausted")
	}

	_, _, _, me, _, _ := fwd.Stats()
	if me != 1 {
		t.Errorf("metricsErr = %d, want 1", me)
	}
}

func TestForwardSecureEndpoint(t *testing.T) {
	fwd := New("localhost:4317", false, slog.Default())
	if fwd.endpoint != "https://localhost:4317" {
		t.Errorf("endpoint = %q, want https://localhost:4317", fwd.endpoint)
	}
}

func TestForwardInsecureEndpoint(t *testing.T) {
	fwd := New("localhost:4317", true, slog.Default())
	if fwd.endpoint != "http://localhost:4317" {
		t.Errorf("endpoint = %q, want http://localhost:4317", fwd.endpoint)
	}
}

func TestForwardClientTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(35 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	fwd := New(ts.URL[len("http://"):], true, slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	metrics := pmetric.NewMetrics()
	protoBytes, _ := pmetricotlp.NewExportRequestFromMetrics(metrics).MarshalProto()

	err := fwd.ForwardMetrics(ctx, protoBytes)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
