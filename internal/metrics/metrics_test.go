package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := New(registry)
	if m == nil {
		t.Fatal("New returned nil")
	}
	if m.SignalsReceived == nil {
		t.Error("SignalsReceived should not be nil")
	}
	if m.DictionaryEntries == nil {
		t.Error("DictionaryEntries should not be nil")
	}
	if m.PipelineRingBufferSize == nil {
		t.Error("PipelineRingBufferSize should not be nil")
	}
	if m.APIRequests == nil {
		t.Error("APIRequests should not be nil")
	}
}

func TestMetricsCounterIncrement(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := New(registry)

	m.SignalsReceived.WithLabelValues("metric", "http").Add(5)
	m.DictionaryAttributesAdded.Add(3)
	m.PipelineDrops.Add(1)

	metrics, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather error: %v", err)
	}
	if len(metrics) == 0 {
		t.Error("expected metrics after increment")
	}
}
