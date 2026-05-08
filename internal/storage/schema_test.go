package storage

import (
	"testing"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
)

func TestEntryToBytesBytesToEntryRoundtrip(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	entry := &dictionary.AttributeEntry{
		Name:        "http.request.method",
		Type:        "string",
		SignalTypes: []dictionary.SignalType{dictionary.SignalTypeMetric, dictionary.SignalTypeTrace},
		FirstSeen:   now,
		LastSeen:    now,
		Status:      dictionary.StatusActive,
		Cardinality: 42,
	}

	data, err := EntryToBytes(entry)
	if err != nil {
		t.Fatalf("EntryToBytes error: %v", err)
	}

	got, err := BytesToEntry(data)
	if err != nil {
		t.Fatalf("BytesToEntry error: %v", err)
	}

	if got.Name != entry.Name {
		t.Errorf("Name = %q, want %q", got.Name, entry.Name)
	}
	if got.Type != entry.Type {
		t.Errorf("Type = %q, want %q", got.Type, entry.Type)
	}
	if got.Cardinality != entry.Cardinality {
		t.Errorf("Cardinality = %d, want %d", got.Cardinality, entry.Cardinality)
	}
	if got.Status != entry.Status {
		t.Errorf("Status = %q, want %q", got.Status, entry.Status)
	}
	if len(got.SignalTypes) != len(entry.SignalTypes) {
		t.Errorf("SignalTypes len = %d, want %d", len(got.SignalTypes), len(entry.SignalTypes))
	}
}

func TestStorageKeyString(t *testing.T) {
	key := StorageKey{SignalType: "metric", AttributeName: "http.method"}
	expected := "metric:http.method"
	if key.String() != expected {
		t.Errorf("String() = %q, want %q", key.String(), expected)
	}
}

func TestParseKey(t *testing.T) {
	signalType, attrName := ParseKey("metric:http.request.method")
	if signalType != "metric" {
		t.Errorf("signalType = %q, want %q", signalType, "metric")
	}
	if attrName != "http.request.method" {
		t.Errorf("attrName = %q, want %q", attrName, "http.request.method")
	}
}

func TestParseKeyNoColon(t *testing.T) {
	signalType, attrName := ParseKey("nocolonkey")
	if signalType != "" {
		t.Errorf("signalType = %q, want empty", signalType)
	}
	if attrName != "nocolonkey" {
		t.Errorf("attrName = %q, want %q", attrName, "nocolonkey")
	}
}

func TestBytesToEntryInvalid(t *testing.T) {
	_, err := BytesToEntry([]byte("not-valid-msgpack"))
	if err == nil {
		t.Error("expected error for invalid data")
	}
}

func BenchmarkEntryToBytes(b *testing.B) {
	entry := &dictionary.AttributeEntry{
		Name:        "http.request.method",
		Type:        "string",
		SignalTypes: []dictionary.SignalType{dictionary.SignalTypeMetric},
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Status:      dictionary.StatusActive,
		Cardinality: 100,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EntryToBytes(entry)
	}
}
