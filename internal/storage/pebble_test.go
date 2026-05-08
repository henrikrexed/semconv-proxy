package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"log/slog"
)

func TestPersisterRoundtrip(t *testing.T) {
	dir := t.TempDir()
	logger := slog.Default()

	p, err := NewPersister(dir, 50*time.Millisecond, 10, logger)
	if err != nil {
		t.Fatalf("NewPersister error: %v", err)
	}

	ctx := context.Background()
	p.Start(ctx)

	entry := &dictionary.AttributeEntry{
		Name:        "test.attr",
		Type:        "string",
		SignalTypes: []dictionary.SignalType{dictionary.SignalTypeMetric},
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Status:      dictionary.StatusActive,
		Cardinality: 5,
	}
	p.Enqueue(entry)

	time.Sleep(200 * time.Millisecond)
	p.Stop()

	p2, err := NewPersister(dir, 50*time.Millisecond, 10, logger)
	if err != nil {
		t.Fatalf("NewPersister (reload) error: %v", err)
	}

	entries, err := p2.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("LoadAll returned %d entries, want 1", len(entries))
	}
	if entries[0].Name != "test.attr" {
		t.Errorf("Name = %q, want %q", entries[0].Name, "test.attr")
	}

	p2.Stop()
}

func TestPersisterEmptySignalTypes(t *testing.T) {
	dir := t.TempDir()
	logger := slog.Default()

	p, err := NewPersister(dir, 50*time.Millisecond, 10, logger)
	if err != nil {
		t.Fatalf("NewPersister error: %v", err)
	}

	ctx := context.Background()
	p.Start(ctx)

	entry := &dictionary.AttributeEntry{
		Name:        "no.signal.type",
		Type:        "string",
		SignalTypes: []dictionary.SignalType{},
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Status:      dictionary.StatusActive,
		Cardinality: 1,
	}
	p.Enqueue(entry)

	time.Sleep(200 * time.Millisecond)
	p.Stop()
}

func TestPersisterLoadEmptyDir(t *testing.T) {
	dir := t.TempDir()
	logger := slog.Default()

	p, err := NewPersister(dir, 50*time.Millisecond, 10, logger)
	if err != nil {
		t.Fatalf("NewPersister error: %v", err)
	}

	entries, err := p.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("LoadAll returned %d entries, want 0", len(entries))
	}

	p.Stop()
}

func TestNewPersisterCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "pebble")
	logger := slog.Default()
	_ = os.RemoveAll(filepath.Join(t.TempDir(), "nested"))

	p, err := NewPersister(dir, 50*time.Millisecond, 10, logger)
	if err != nil {
		t.Fatalf("NewPersister error: %v", err)
	}
	p.Stop()
}
