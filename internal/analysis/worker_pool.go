package analysis

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"github.com/henrikrexed/semconv-proxy/internal/metrics"
)

type WorkerPool struct {
	workers   int
	tasks     <-chan *AnalysisTask
	dict      *dictionary.Dictionary
	m         *metrics.Metrics
	wg        sync.WaitGroup
	cancel    context.CancelFunc
	processed atomic.Int64
}

func NewWorkerPool(workers int, tasks <-chan *AnalysisTask, dict *dictionary.Dictionary) *WorkerPool {
	return NewWorkerPoolWithMetrics(workers, tasks, dict, nil)
}

func NewWorkerPoolWithMetrics(workers int, tasks <-chan *AnalysisTask, dict *dictionary.Dictionary, m *metrics.Metrics) *WorkerPool {
	return &WorkerPool{
		workers: workers,
		tasks:   tasks,
		dict:    dict,
		m:       m,
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
		change := wp.dict.Upsert(entry)
		if wp.m != nil {
			switch change {
			case dictionary.ChangeAdded:
				wp.m.DictionaryAttributesAdded.Inc()
			case dictionary.ChangeTypeModified:
				wp.m.DictionaryAttributesChanged.Inc()
			}
		}
	}
}
