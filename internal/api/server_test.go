package api

import (
	"context"
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

func TestServerStartStop(t *testing.T) {
	dict, _ := dictionary.New(&dictionary.Config{ShardCount: 4, GlobalBudget: 100, PerAttrCap: 10}, slog.Default())
	tracker := cardinality.NewTracker(100, 10, slog.Default())
	weaverExporter := export.NewWeaverExporter()
	healthAgg := health.NewAggregator(slog.Default())
	registry := prometheus.NewRegistry()
	m := metrics.New(registry)

	s := NewServer(0, dict, tracker, weaverExporter, slog.Default(), healthAgg, registry, m)

	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	if !s.ready {
		t.Error("expected ready after Start")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Stop(stopCtx)
}

func TestServerSetReady(t *testing.T) {
	s := &Server{ready: false}
	s.SetReady(true)
	if !s.ready {
		t.Error("expected ready=true after SetReady(true)")
	}
	s.SetReady(false)
	if s.ready {
		t.Error("expected ready=false after SetReady(false)")
	}
}
