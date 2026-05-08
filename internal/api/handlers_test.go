package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"log/slog"

	"github.com/henrikrexed/semconv-proxy/internal/cardinality"
	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"github.com/henrikrexed/semconv-proxy/internal/export"
	"github.com/henrikrexed/semconv-proxy/internal/health"
	"github.com/henrikrexed/semconv-proxy/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dict, err := dictionary.New(&dictionary.Config{
		ShardCount:   4,
		GlobalBudget: 1000,
		PerAttrCap:   100,
	}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}

	dict.Upsert(&dictionary.AttributeEntry{
		Name:        "http.request.method",
		Type:        "string",
		SignalTypes: []dictionary.SignalType{dictionary.SignalTypeMetric},
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Status:      dictionary.StatusActive,
		Cardinality: 10,
	})

	tracker := cardinality.NewTracker(1000, 100, slog.Default())
	weaverExporter := export.NewWeaverExporter()
	healthAgg := health.NewAggregator(slog.Default())
	healthAgg.Register("test")
	healthAgg.Update("test", health.StatusOK)
	registry := prometheus.NewRegistry()
	m := metrics.New(registry)

	return NewServer(0, dict, tracker, weaverExporter, slog.Default(), healthAgg, registry, m)
}

func TestHandleHealthz(t *testing.T) {
	s := newTestServer(t)
	s.ready = true

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	s.handleHealthz(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["status"] != "alive" {
		t.Errorf("status = %q, want %q", body["status"], "alive")
	}
}

func TestHandleReadyzReady(t *testing.T) {
	s := newTestServer(t)
	s.ready = true

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	s.handleReadyz(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleReadyzNotReady(t *testing.T) {
	s := newTestServer(t)
	s.ready = false

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	s.handleReadyz(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleDictionary(t *testing.T) {
	s := newTestServer(t)
	s.ready = true

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dictionary", nil)
	w := httptest.NewRecorder()
	s.handleDictionary(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["total"] == nil {
		t.Error("expected total field")
	}
}

func TestHandleDictionaryEntry(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dictionary/http.request.method", nil)
	w := httptest.NewRecorder()
	s.handleDictionaryEntry(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleDictionaryEntryNotFound(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dictionary/nonexistent", nil)
	w := httptest.NewRecorder()
	s.handleDictionaryEntry(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleExport(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/export?format=weaver", nil)
	w := httptest.NewRecorder()
	s.handleExport(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleExportBadFormat(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/export?format=csv", nil)
	w := httptest.NewRecorder()
	s.handleExport(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
