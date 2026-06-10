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

type cardinalityResp struct {
	GlobalBudget struct {
		Used           int     `json:"used"`
		Limit          int     `json:"limit"`
		UtilizationPct float64 `json:"utilization_pct"`
	} `json:"global_budget"`
	Attributes []struct {
		Name           string  `json:"name"`
		Cardinality    int64   `json:"cardinality"`
		Cap            int     `json:"cap"`
		UtilizationPct float64 `json:"utilization_pct"`
	} `json:"attributes"`
}

func doCardinality(t *testing.T, s *Server, query string) cardinalityResp {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/semconv/cardinality"+query, nil)
	w := httptest.NewRecorder()
	s.handleCardinality(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var res cardinalityResp
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal response: %v (raw=%s)", err, w.Body.Bytes())
	}
	return res
}

func TestHandleCardinalityMethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/semconv/cardinality", nil)
	w := httptest.NewRecorder()
	s.handleCardinality(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleCardinalityGlobalBudgetAndAttrs(t *testing.T) {
	s := newTestServer(t)
	for _, v := range []string{"a", "b", "c"} {
		s.tracker.TrackValue("svc.instance.id", v)
	}

	res := doCardinality(t, s, "?threshold=2")

	if res.GlobalBudget.Used != 1 {
		t.Errorf("used = %d, want 1 (one distinct attr tracked)", res.GlobalBudget.Used)
	}
	if res.GlobalBudget.Limit != 1000 {
		t.Errorf("limit = %d, want 1000", res.GlobalBudget.Limit)
	}
	if res.GlobalBudget.UtilizationPct <= 0 {
		t.Errorf("utilization_pct = %v, want > 0", res.GlobalBudget.UtilizationPct)
	}
	if len(res.Attributes) != 1 {
		t.Fatalf("attributes len = %d, want 1", len(res.Attributes))
	}
	a := res.Attributes[0]
	if a.Name != "svc.instance.id" || a.Cardinality != 3 || a.Cap != 100 {
		t.Errorf("attr = %+v, want {svc.instance.id 3 100 ...}", a)
	}
	if a.UtilizationPct != 3 {
		t.Errorf("utilization_pct = %v, want 3 (3/100*100)", a.UtilizationPct)
	}
}

func TestHandleCardinalityThresholdFilters(t *testing.T) {
	s := newTestServer(t)
	for _, v := range []string{"a", "b", "c", "d", "e"} {
		s.tracker.TrackValue("high.attr", v)
	}
	s.tracker.TrackValue("low.attr", "only")

	res := doCardinality(t, s, "?threshold=2")

	if len(res.Attributes) != 1 {
		t.Fatalf("attributes len = %d, want 1 (low.attr below threshold)", len(res.Attributes))
	}
	if res.Attributes[0].Name != "high.attr" {
		t.Errorf("attr = %q, want high.attr", res.Attributes[0].Name)
	}
	if res.GlobalBudget.Used != 2 {
		t.Errorf("used = %d, want 2 (both attrs counted in budget)", res.GlobalBudget.Used)
	}
}

func TestHandleCardinalityInvalidThresholdUsesDefault(t *testing.T) {
	s := newTestServer(t)
	for _, v := range []string{"a", "b", "c"} {
		s.tracker.TrackValue("svc.instance.id", v)
	}

	res := doCardinality(t, s, "?threshold=not-a-number")

	if len(res.Attributes) != 0 {
		t.Errorf("attributes len = %d, want 0 (default threshold 100 filters out card=3)", len(res.Attributes))
	}
	if res.GlobalBudget.Used != 1 {
		t.Errorf("used = %d, want 1", res.GlobalBudget.Used)
	}
}
