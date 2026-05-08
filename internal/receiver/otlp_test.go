package receiver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/analysis"
	"github.com/henrikrexed/semconv-proxy/internal/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"log/slog"
)

func TestHTTPMetricsHandler(t *testing.T) {
	forwarded := atomic.Int32{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded.Add(1)
		w.WriteHeader(http.StatusOK)
		resp, _ := pmetricotlp.NewExportResponse().MarshalProto()
		w.Write(resp)
	}))
	defer ts.Close()

	fwd := exporter.New(ts.URL[len("http://"):], true, slog.Default())
	buf := analysis.NewRingBuffer(100)

	go func() {
		for range buf.Channel() {
		}
	}()

	r := New(0, 0, fwd, buf, slog.Default())

	metrics := pmetric.NewMetrics()
	protoBytes, _ := pmetricotlp.NewExportRequestFromMetrics(metrics).MarshalProto()

	req := httptest.NewRequest(http.MethodPost, "/v1/metrics", bytes.NewReader(protoBytes))
	req.Header.Set("Content-Type", "application/x-protobuf")
	w := httptest.NewRecorder()

	r.handleHTTPMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	time.Sleep(50 * time.Millisecond)
	if forwarded.Load() != 1 {
		t.Errorf("forwarded = %d, want 1", forwarded.Load())
	}
	if buf.Count() != 1 {
		t.Errorf("buffer count = %d, want 1", buf.Count())
	}
}

func TestHTTPTracesHandler(t *testing.T) {
	forwarded := atomic.Int32{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded.Add(1)
		w.WriteHeader(http.StatusOK)
		resp, _ := ptraceotlp.NewExportResponse().MarshalProto()
		w.Write(resp)
	}))
	defer ts.Close()

	fwd := exporter.New(ts.URL[len("http://"):], true, slog.Default())
	buf := analysis.NewRingBuffer(100)

	go func() {
		for range buf.Channel() {
		}
	}()

	r := New(0, 0, fwd, buf, slog.Default())

	traces := ptrace.NewTraces()
	protoBytes, _ := ptraceotlp.NewExportRequestFromTraces(traces).MarshalProto()

	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(protoBytes))
	w := httptest.NewRecorder()

	r.handleHTTPTraces(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHTTPLogsHandler(t *testing.T) {
	forwarded := atomic.Int32{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded.Add(1)
		w.WriteHeader(http.StatusOK)
		resp, _ := plogotlp.NewExportResponse().MarshalProto()
		w.Write(resp)
	}))
	defer ts.Close()

	fwd := exporter.New(ts.URL[len("http://"):], true, slog.Default())
	buf := analysis.NewRingBuffer(100)

	go func() {
		for range buf.Channel() {
		}
	}()

	r := New(0, 0, fwd, buf, slog.Default())

	logs := plog.NewLogs()
	protoBytes, _ := plogotlp.NewExportRequestFromLogs(logs).MarshalProto()

	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewReader(protoBytes))
	w := httptest.NewRecorder()

	r.handleHTTPLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHTTPEmptyForwarder(t *testing.T) {
	buf := analysis.NewRingBuffer(100)
	go func() {
		for range buf.Channel() {
		}
	}()

	r := New(0, 0, nil, buf, slog.Default())

	metrics := pmetric.NewMetrics()
	protoBytes, _ := pmetricotlp.NewExportRequestFromMetrics(metrics).MarshalProto()

	req := httptest.NewRequest(http.MethodPost, "/v1/metrics", bytes.NewReader(protoBytes))
	w := httptest.NewRecorder()

	r.handleHTTPMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHTTPBadBody(t *testing.T) {
	buf := analysis.NewRingBuffer(100)
	r := New(0, 0, nil, buf, slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/v1/metrics", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()
	r.handleHTTPMetrics(w, req)
}
