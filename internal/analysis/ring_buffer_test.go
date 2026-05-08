package analysis

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

func TestRingBuffer(t *testing.T) {
	rb := NewRingBuffer(100)
	task := &AnalysisTask{
		SignalType: SignalMetric,
		Timestamp:  time.Now(),
		Data:       []byte("test"),
	}

	rb.Write(task)
	if rb.Len() != 1 {
		t.Errorf("Len() = %d, want 1", rb.Len())
	}

	select {
	case got := <-rb.Channel():
		if got.SignalType != SignalMetric {
			t.Errorf("got.SignalType = %q, want %q", got.SignalType, SignalMetric)
		}
	default:
		t.Error("Channel() empty after Write()")
	}
}

func TestRingBufferOverflow(t *testing.T) {
	rb := NewRingBuffer(5)
	for i := 0; i < 10; i++ {
		rb.Write(&AnalysisTask{SignalType: SignalMetric, Timestamp: time.Now()})
	}
	if rb.Dropped() == 0 {
		t.Error("expected drops on overflow")
	}
}

func TestWorkerPoolWithMetricsData(t *testing.T) {
	dict, _ := dictionary.New(&dictionary.Config{ShardCount: 4, GlobalBudget: 100, PerAttrCap: 10}, nil)
	ch := make(chan *AnalysisTask, 100)
	ext := NewExtractor()
	wp := NewWorkerPoolWithMetrics(2, ch, dict, nil, nil, ext)

	ctx := context.Background()
	wp.Start(ctx)

	data := buildTestMetricData(t, "test.metric", map[string]string{"test.attr": "value"})
	for i := 0; i < 10; i++ {
		ch <- &AnalysisTask{
			SignalType: SignalMetric,
			Timestamp:  time.Now(),
			Data:       data,
		}
	}
	close(ch)

	time.Sleep(200 * time.Millisecond)
	wp.Stop()

	processed := wp.Processed()
	if processed != 10 {
		t.Errorf("Processed() = %d, want 10", processed)
	}
}

func TestWorkerPoolCancellation(t *testing.T) {
	dict, _ := dictionary.New(&dictionary.Config{ShardCount: 4, GlobalBudget: 100, PerAttrCap: 10}, nil)
	ch := make(chan *AnalysisTask, 100)
	ext := NewExtractor()
	wp := NewWorkerPoolWithMetrics(2, ch, dict, nil, nil, ext)

	ctx, cancel := context.WithCancel(context.Background())
	wp.Start(ctx)

	data := buildTestMetricData(t, "cancel.metric", map[string]string{"cancel.attr": "val"})

	var wrote atomic.Int32
	go func() {
		for i := 0; i < 1000; i++ {
			ch <- &AnalysisTask{
				SignalType: SignalMetric,
				Timestamp:  time.Now(),
				Data:       data,
			}
			wrote.Add(1)
		}
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	wp.Stop()

	if wrote.Load() == 0 {
		t.Error("no tasks written")
	}
}

func TestExtractorMetrics(t *testing.T) {
	e := NewExtractor()
	attrs := map[string]string{
		"http.method": "GET",
		"status.code": "200",
		"server.name": "api",
	}
	data := buildTestMetricData(t, "http.requests", attrs)

	extracted, err := e.ExtractFromData("metric", data)
	if err != nil {
		t.Fatalf("ExtractFromData error: %v", err)
	}
	if len(extracted) == 0 {
		t.Error("expected extracted attributes, got none")
	}

	names := make(map[string]bool)
	for _, a := range extracted {
		names[a.Name] = true
	}
	if !names["http.requests"] {
		t.Error("expected metric name 'http.requests' in extracted attrs")
	}
}

func TestExtractorTraces(t *testing.T) {
	e := NewExtractor()
	data := buildTestTraceData(t, map[string]string{"http.method": "GET", "span.kind": "server"})

	extracted, err := e.ExtractFromData("trace", data)
	if err != nil {
		t.Fatalf("ExtractFromData error: %v", err)
	}
	if len(extracted) == 0 {
		t.Error("expected extracted attributes from trace, got none")
	}
}

func TestExtractorLogs(t *testing.T) {
	e := NewExtractor()
	data := buildTestLogData(t, map[string]string{"log.level": "info", "service.name": "api"})

	extracted, err := e.ExtractFromData("log", data)
	if err != nil {
		t.Fatalf("ExtractFromData error: %v", err)
	}
	if len(extracted) == 0 {
		t.Error("expected extracted attributes from log, got none")
	}
}

func TestExtractorUnknownSignalType(t *testing.T) {
	e := NewExtractor()
	_, err := e.ExtractFromData("unknown", []byte{})
	if err == nil {
		t.Error("expected error for unknown signal type")
	}
}

func TestToDictionaryEntries(t *testing.T) {
	now := time.Now()
	attrs := []ExtractedAttr{
		{Name: "http.method", Type: "string", SignalType: "metric"},
		{Name: "status.code", Type: "int", SignalType: "trace"},
	}
	entries := ToDictionaryEntries(attrs, now)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "http.method" {
		t.Errorf("entries[0].Name = %q, want 'http.method'", entries[0].Name)
	}
	if entries[0].SignalTypes[0] != dictionary.SignalTypeMetric {
		t.Errorf("entries[0].SignalTypes[0] = %q, want 'metric'", entries[0].SignalTypes[0])
	}
	if entries[0].FirstSeen != now {
		t.Error("FirstSeen mismatch")
	}
}

func BenchmarkExtractorMetrics(b *testing.B) {
	e := NewExtractor()
	attrs := map[string]string{
		"http.method":     "GET",
		"status.code":     "200",
		"server.name":     "api",
		"service.version": "1.0.0",
	}
	data := buildTestMetricData(&testing.T{}, "http.requests", attrs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := e.ExtractFromData("metric", data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExtractorTraces(b *testing.B) {
	e := NewExtractor()
	data := buildTestTraceData(&testing.T{}, map[string]string{
		"http.method": "GET",
		"span.kind":   "server",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := e.ExtractFromData("trace", data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExtractorLogs(b *testing.B) {
	e := NewExtractor()
	data := buildTestLogData(&testing.T{}, map[string]string{
		"log.level":    "info",
		"service.name": "api",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := e.ExtractFromData("log", data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWorkerPoolProcess(b *testing.B) {
	dict, _ := dictionary.New(&dictionary.Config{ShardCount: 4, GlobalBudget: 10000, PerAttrCap: 1000}, nil)
	ch := make(chan *AnalysisTask, b.N)
	ext := NewExtractor()
	wp := NewWorkerPoolWithMetrics(4, ch, dict, nil, nil, ext)

	ctx := context.Background()
	wp.Start(ctx)

	data := buildTestMetricData(&testing.T{}, "bench.metric", map[string]string{
		"attr1": "val1",
		"attr2": "val2",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch <- &AnalysisTask{
			SignalType: SignalMetric,
			Timestamp:  time.Now(),
			Data:       data,
		}
	}
	close(ch)
	wp.Stop()
}

func BenchmarkRingBufferWrite(b *testing.B) {
	rb := NewRingBuffer(b.N + 1)
	task := &AnalysisTask{
		SignalType: SignalMetric,
		Timestamp:  time.Now(),
		Data:       make([]byte, 100),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Write(task)
	}
}

func buildTestMetricData(t *testing.T, name string, attrs map[string]string) []byte {
	t.Helper()
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName(name)
	m.SetEmptySum()
	dp := m.Sum().DataPoints().AppendEmpty()
	dp.SetIntValue(1)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	for k, v := range attrs {
		dp.Attributes().PutStr(k, v)
	}

	req := pmetricotlp.NewExportRequestFromMetrics(metrics)
	data, err := req.MarshalProto()
	if err != nil {
		t.Fatalf("failed to marshal test metric data: %v", err)
	}
	return data
}

func buildTestTraceData(t *testing.T, attrs map[string]string) []byte {
	t.Helper()
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")
	for k, v := range attrs {
		span.Attributes().PutStr(k, v)
	}

	req := ptraceotlp.NewExportRequestFromTraces(traces)
	data, err := req.MarshalProto()
	if err != nil {
		t.Fatalf("failed to marshal test trace data: %v", err)
	}
	return data
}

func buildTestLogData(t *testing.T, attrs map[string]string) []byte {
	t.Helper()
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	record := sl.LogRecords().AppendEmpty()
	record.Body().SetStr("test log message")
	for k, v := range attrs {
		record.Attributes().PutStr(k, v)
	}

	req := plogotlp.NewExportRequestFromLogs(logs)
	data, err := req.MarshalProto()
	if err != nil {
		t.Fatalf("failed to marshal test log data: %v", err)
	}
	return data
}
