package analysis

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
)

type SignalType string

const (
	SignalMetric SignalType = "metric"
	SignalTrace  SignalType = "trace"
	SignalLog    SignalType = "log"
)

type AnalysisTask struct {
	SignalType SignalType      `json:"signal_type"`
	Timestamp  time.Time       `json:"timestamp"`
	Data       []byte          `json:"data"`
	Attributes []ExtractedAttr `json:"attributes,omitempty"`
}

type ExtractedAttr struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	SignalType  string `json:"signal_type"`
	Cardinality int64  `json:"cardinality"`
}

type RingBuffer struct {
	mu       sync.Mutex
	buf      []*AnalysisTask
	capacity int
	count    atomic.Int64
	dropped  atomic.Int64
	writeCh  chan *AnalysisTask
}

func NewRingBuffer(capacity int) *RingBuffer {
	rb := &RingBuffer{
		buf:      make([]*AnalysisTask, capacity),
		capacity: capacity,
		writeCh:  make(chan *AnalysisTask, capacity),
	}
	return rb
}

func (rb *RingBuffer) Write(task *AnalysisTask) {
	rb.count.Add(1)
	select {
	case rb.writeCh <- task:
	default:
		rb.dropped.Add(1)
		rb.mu.Lock()
		select {
		case rb.writeCh <- task:
		default:
		}
		rb.mu.Unlock()
	}
}

func (rb *RingBuffer) Channel() <-chan *AnalysisTask {
	return rb.writeCh
}

func (rb *RingBuffer) Count() int64 {
	return rb.count.Load()
}

func (rb *RingBuffer) Dropped() int64 {
	return rb.dropped.Load()
}

func (rb *RingBuffer) Len() int {
	return len(rb.writeCh)
}

type WorkerPool struct {
	workers   int
	tasks     <-chan *AnalysisTask
	dict      *dictionary.Dictionary
	wg        sync.WaitGroup
	cancel    context.CancelFunc
	processed atomic.Int64
}

func NewWorkerPool(workers int, tasks <-chan *AnalysisTask, dict *dictionary.Dictionary) *WorkerPool {
	return &WorkerPool{
		workers: workers,
		tasks:   tasks,
		dict:    dict,
	}
}

func (wp *WorkerPool) Start(ctx context.Context) {
	ctx, wp.cancel = context.WithCancel(ctx)
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.work(ctx, i)
	}
}

func (wp *WorkerPool) Stop() {
	if wp.cancel != nil {
		wp.cancel()
	}
	wp.wg.Wait()
}

func (wp *WorkerPool) Processed() int64 {
	return wp.processed.Load()
}

func (wp *WorkerPool) work(ctx context.Context, id int) {
	defer wp.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-wp.tasks:
			if !ok {
				return
			}
			wp.processTask(task)
			wp.processed.Add(1)
		}
	}
}

func (wp *WorkerPool) processTask(task *AnalysisTask) {
	now := time.Now()
	for _, attr := range task.Attributes {
		entry := &dictionary.AttributeEntry{
			Name:        attr.Name,
			Type:        attr.Type,
			SignalTypes: []dictionary.SignalType{dictionary.SignalType(attr.SignalType)},
			FirstSeen:   now,
			LastSeen:    now,
			Status:      dictionary.StatusActive,
			Cardinality: attr.Cardinality,
		}
		wp.dict.Upsert(entry)
	}
}
