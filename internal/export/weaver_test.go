package export

import (
	"strings"
	"testing"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
)

func TestWeaverExportBasic(t *testing.T) {
	exporter := NewWeaverExporter()
	now := time.Now()

	entries := []*dictionary.AttributeEntry{
		{
			Name:        "http.request.method",
			Type:        "string",
			SignalTypes: []dictionary.SignalType{dictionary.SignalTypeMetric},
			FirstSeen:   now,
			LastSeen:    now,
			Status:      dictionary.StatusActive,
			Cardinality: 5,
		},
		{
			Name:        "http.response.status_code",
			Type:        "int",
			SignalTypes: []dictionary.SignalType{dictionary.SignalTypeTrace},
			FirstSeen:   now,
			LastSeen:    now,
			Status:      dictionary.StatusActive,
			Cardinality: 10,
		},
	}

	yaml, err := exporter.Export(entries)
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}

	output := string(yaml)
	if !strings.Contains(output, "http.request.method") {
		t.Error("expected http.request.method in output")
	}
	if !strings.Contains(output, "http.response.status_code") {
		t.Error("expected http.response.status_code in output")
	}
}

func TestWeaverExportEmpty(t *testing.T) {
	exporter := NewWeaverExporter()
	yaml, err := exporter.Export([]*dictionary.AttributeEntry{})
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}
	if len(yaml) == 0 {
		t.Error("expected non-empty output for empty entries")
	}
}

func TestWeaverExportNil(t *testing.T) {
	exporter := NewWeaverExporter()
	yaml, err := exporter.Export(nil)
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}
	if len(yaml) == 0 {
		t.Error("expected non-empty output for nil entries")
	}
}
