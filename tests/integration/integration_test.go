//go:build !docker

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/analysis"
	"github.com/henrikrexed/semconv-proxy/internal/api"
	"github.com/henrikrexed/semconv-proxy/internal/cardinality"
	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"github.com/henrikrexed/semconv-proxy/internal/export"
	"github.com/henrikrexed/semconv-proxy/internal/exporter"
	"github.com/henrikrexed/semconv-proxy/internal/health"
	"github.com/henrikrexed/semconv-proxy/internal/metrics"
	"github.com/henrikrexed/semconv-proxy/internal/receiver"
	"github.com/henrikrexed/semconv-proxy/internal/storage"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"log/slog"
)

type testEnv struct {
	httpPort    int
	grpcPort    int
	apiPort     int
	backendURL  string
	fwd         *exporter.Forwarder
	dict        *dictionary.Dictionary
	cancel      context.CancelFunc
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	httpPort := 14318
	grpcPort := 14317
	apiPort := 18080

	backendReceived := make(chan string, 100)
	backendServer := &http.Server{
		Addr: fmt.Sprintf(":%d", 14418),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			backendReceived <- r.URL.Path
			_ = body
			switch r.URL.Path {
			case "/v1/metrics":
				resp, _ := pmetricotlp.NewExportResponse().MarshalProto()
				w.Write(resp)
			case "/v1/traces":
				resp, _ := ptraceotlp.NewExportResponse().MarshalProto()
				w.Write(resp)
			case "/v1/logs":
				resp, _ := plogotlp.NewExportResponse().MarshalProto()
				w.Write(resp)
			}
		}),
	}
	ln, _ := new(net.ListenConfig).Listen(context.Background(), "tcp", backendServer.Addr)
	go backendServer.Serve(ln)
	t.Cleanup(func() { backendServer.Close() })

	fwd := exporter.New(fmt.Sprintf("127.0.0.1:%d", 14418), true, logger)

	dict, _ := dictionary.New(&dictionary.Config{
		ShardCount:   4,
		GlobalBudget: 10000,
		PerAttrCap:   1000,
	}, logger)

	tracker := cardinality.NewTracker(10000, 1000, logger)
	weaverExporter := export.NewWeaverExporter()
	healthAgg := health.NewAggregator(logger)
	registry := prometheus.NewRegistry()
	_ = metrics.New(registry)

	buf := analysis.NewRingBuffer(10000)
	wp := analysis.NewWorkerPool(4, buf.Channel(), dict)

	ctx, cancel := context.WithCancel(context.Background())
	wp.Start(ctx)

	recv := receiver.New(httpPort, grpcPort, fwd, buf, logger)
	if err := recv.Start(ctx); err != nil {
		t.Fatalf("receiver start: %v", err)
	}
	healthAgg.Register("receiver")
	healthAgg.Update("receiver", health.StatusOK)

	apiServer := api.NewServer(apiPort, dict, tracker, weaverExporter, logger, healthAgg, registry)
	if err := apiServer.Start(ctx); err != nil {
		t.Fatalf("api start: %v", err)
	}

	t.Cleanup(func() {
		cancel()
		wp.Stop()
		recv.Stop(context.Background())
		apiServer.Stop(context.Background())
	})

	return &testEnv{
		httpPort:   httpPort,
		grpcPort:   grpcPort,
		apiPort:    apiPort,
		backendURL: fmt.Sprintf("http://127.0.0.1:%d", 14418),
		fwd:        fwd,
		dict:       dict,
		cancel:     cancel,
	}
}

func TestSignalForwardingHTTP(t *testing.T) {
	env := setupTestEnv(t)

	tests := []struct {
		name    string
		path    string
		makeReq func() []byte
	}{
		{
			name: "metrics",
			path: "/v1/metrics",
			makeReq: func() []byte {
				metrics := pmetric.NewMetrics()
				req := pmetricotlp.NewExportRequestFromMetrics(metrics)
				b, _ := req.MarshalProto()
				return b
			},
		},
		{
			name: "traces",
			path: "/v1/traces",
			makeReq: func() []byte {
				traces := ptrace.NewTraces()
				req := ptraceotlp.NewExportRequestFromTraces(traces)
				b, _ := req.MarshalProto()
				return b
			},
		},
		{
			name: "logs",
			path: "/v1/logs",
			makeReq: func() []byte {
				logs := plog.NewLogs()
				req := plogotlp.NewExportRequestFromLogs(logs)
				b, _ := req.MarshalProto()
				return b
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.makeReq()
			resp, err := http.Post(
				fmt.Sprintf("http://127.0.0.1:%d%s", env.httpPort, tc.path),
				"application/x-protobuf",
				bytes.NewReader(body),
			)
			if err != nil {
				t.Fatalf("POST error: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
		})
	}
}

func TestDictionaryLifecycle(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	dict, _ := dictionary.New(&dictionary.Config{
		ShardCount:   4,
		GlobalBudget: 100,
		PerAttrCap:   10,
	}, logger)

	dict.Upsert(&dictionary.AttributeEntry{
		Name:        "test.attr.1",
		Type:        "string",
		SignalTypes: []dictionary.SignalType{dictionary.SignalTypeMetric},
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Status:      dictionary.StatusActive,
		Cardinality: 5,
	})

	if dict.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", dict.Count())
	}

	entry, ok := dict.Get("test.attr.1")
	if !ok {
		t.Fatal("Get() returned not found")
	}
	if entry.Name != "test.attr.1" {
		t.Errorf("Name = %q, want %q", entry.Name, "test.attr.1")
	}

	dict.Delete("test.attr.1")
	if dict.Count() != 0 {
		t.Errorf("Count() after delete = %d, want 0", dict.Count())
	}

	_, ok = dict.Get("test.attr.1")
	if ok {
		t.Error("Get() should return false after delete")
	}
}

func TestDictionaryStalenessAndPurge(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	dict, _ := dictionary.New(&dictionary.Config{
		ShardCount:   4,
		GlobalBudget: 100,
		PerAttrCap:   10,
	}, logger)

	now := time.Now()
	dict.Upsert(&dictionary.AttributeEntry{
		Name:        "old.attr",
		Type:        "string",
		SignalTypes: []dictionary.SignalType{dictionary.SignalTypeMetric},
		FirstSeen:   now.Add(-48 * time.Hour),
		LastSeen:    now.Add(-48 * time.Hour),
		Status:      dictionary.StatusActive,
		Cardinality: 1,
	})

	stale := dict.MarkStale(now, 24*time.Hour)
	if stale != 1 {
		t.Errorf("MarkStale = %d, want 1", stale)
	}

	purged := dict.PurgeExpired(now, 47*time.Hour)
	if purged != 1 {
		t.Errorf("PurgeExpired = %d, want 1", purged)
	}

	if dict.Count() != 0 {
		t.Errorf("Count() after purge = %d, want 0", dict.Count())
	}
}

func TestDictionaryGlobalBudget(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	dict, _ := dictionary.New(&dictionary.Config{
		ShardCount:   4,
		GlobalBudget: 2,
		PerAttrCap:   10,
	}, logger)

	dict.Upsert(&dictionary.AttributeEntry{Name: "attr.1", Type: "string", SignalTypes: []dictionary.SignalType{dictionary.SignalTypeMetric}, FirstSeen: time.Now(), LastSeen: time.Now(), Status: dictionary.StatusActive})
	dict.Upsert(&dictionary.AttributeEntry{Name: "attr.2", Type: "string", SignalTypes: []dictionary.SignalType{dictionary.SignalTypeMetric}, FirstSeen: time.Now(), LastSeen: time.Now(), Status: dictionary.StatusActive})
	dict.Upsert(&dictionary.AttributeEntry{Name: "attr.3", Type: "string", SignalTypes: []dictionary.SignalType{dictionary.SignalTypeMetric}, FirstSeen: time.Now(), LastSeen: time.Now(), Status: dictionary.StatusActive})

	if dict.Count() != 2 {
		t.Errorf("Count() = %d, want 2 (budget limit)", dict.Count())
	}
}

func TestWeaverExport(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	dict, _ := dictionary.New(&dictionary.Config{
		ShardCount:   4,
		GlobalBudget: 100,
		PerAttrCap:   10,
	}, logger)

	now := time.Now()
	dict.Upsert(&dictionary.AttributeEntry{
		Name:        "http.request.method",
		Type:        "string",
		SignalTypes: []dictionary.SignalType{dictionary.SignalTypeMetric},
		FirstSeen:   now,
		LastSeen:    now,
		Status:      dictionary.StatusActive,
		Cardinality: 5,
	})

	weaverExporter := export.NewWeaverExporter()
	entries := dict.List(nil)
	yaml, err := weaverExporter.Export(entries)
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}

	output := string(yaml)
	if len(output) == 0 {
		t.Error("expected non-empty YAML output")
	}
}

func TestPebblePersistenceRoundtrip(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	persister, err := storage.NewPersister(dir, 50*time.Millisecond, 10, logger)
	if err != nil {
		t.Fatalf("NewPersister error: %v", err)
	}

	ctx := context.Background()
	persister.Start(ctx)

	entry := &dictionary.AttributeEntry{
		Name:        "persist.test",
		Type:        "string",
		SignalTypes: []dictionary.SignalType{dictionary.SignalTypeMetric},
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Status:      dictionary.StatusActive,
		Cardinality: 42,
	}
	persister.Enqueue(entry)

	time.Sleep(200 * time.Millisecond)
	persister.Stop()

	p2, _ := storage.NewPersister(dir, 50*time.Millisecond, 10, logger)
	entries, err := p2.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].Name != "persist.test" {
		t.Errorf("Name = %q, want %q", entries[0].Name, "persist.test")
	}
	p2.Stop()
}

func TestHealthEndpoint(t *testing.T) {
	env := setupTestEnv(t)
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", env.apiPort))
	if err != nil {
		t.Fatalf("GET healthz error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthz status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "alive" {
		t.Errorf("healthz status = %q, want %q", body["status"], "alive")
	}
}

func TestMetricsEndpoint(t *testing.T) {
	env := setupTestEnv(t)
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", env.apiPort))
	if err != nil {
		t.Fatalf("GET /metrics error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("/metrics status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	output := string(body)
	if len(output) == 0 {
		t.Error("expected non-empty /metrics response")
	}
}

func TestReadyEndpoint(t *testing.T) {
	env := setupTestEnv(t)
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/readyz", env.apiPort))
	if err != nil {
		t.Fatalf("GET readyz error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("readyz status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
