package analysis

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
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

func TestWorkerPool(t *testing.T) {
	dict, _ := dictionary.New(&dictionary.Config{ShardCount: 4, GlobalBudget: 100, PerAttrCap: 10}, nil)
	ch := make(chan *AnalysisTask, 100)
	wp := NewWorkerPool(2, ch, dict)

	ctx := context.Background()
	wp.Start(ctx)

	for i := 0; i < 10; i++ {
		ch <- &AnalysisTask{
			SignalType: SignalMetric,
			Timestamp:  time.Now(),
			Attributes: []ExtractedAttr{
				{Name: fmt.Sprintf("test.attr.%d", i), Type: "string", SignalType: "metric"},
			},
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
	wp := NewWorkerPool(2, ch, dict)

	ctx, cancel := context.WithCancel(context.Background())
	wp.Start(ctx)

	var wrote atomic.Int32
	go func() {
		for i := 0; i < 1000; i++ {
			ch <- &AnalysisTask{
				SignalType: SignalMetric,
				Timestamp:  time.Now(),
				Attributes: []ExtractedAttr{{Name: "cancel.test", Type: "string", SignalType: "metric"}},
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

func TestExtractor(t *testing.T) {
	e := NewExtractor()
	attrs := e.ExtractMetricAttributes("http.requests", "counter", "1", "cumulative", map[string]string{
		"http.method": "string",
		"status.code": "int",
	})
	if len(attrs) != 3 {
		t.Errorf("ExtractMetricAttributes count = %d, want 3", len(attrs))
	}

	task := e.ToAnalysisTask("metric", attrs)
	if task.SignalType != SignalMetric {
		t.Errorf("task.SignalType = %q, want %q", task.SignalType, SignalMetric)
	}

	entries := e.ToDictionaryEntries(task)
	if len(entries) != 3 {
		t.Errorf("entries count = %d, want 3", len(entries))
	}
}

func BenchmarkRingBufferWrite(b *testing.B) {
	rb := NewRingBuffer(10000)
	go func() {
		for range rb.Channel() {
		}
	}()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Write(&AnalysisTask{SignalType: SignalMetric, Timestamp: time.Now()})
	}
}
