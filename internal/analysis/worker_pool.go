package analysis

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/henrikrexed/semconv-proxy/internal/cardinality"
	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"github.com/henrikrexed/semconv-proxy/internal/metrics"
)

type WorkerPool struct {
	workers   int
	tasks     <-chan *AnalysisTask
	dict      *dictionary.Dictionary
	tracker   *cardinality.Tracker
	extractor *Extractor
	m         *metrics.Metrics
	wg        sync.WaitGroup
	cancel    context.CancelFunc
	processed atomic.Int64
}

func NewWorkerPool(workers int, tasks <-chan *AnalysisTask, dict *dictionary.Dictionary) *WorkerPool {
	return NewWorkerPoolWithMetrics(workers, tasks, dict, nil, nil, nil)
}

func NewWorkerPoolWithMetrics(workers int, tasks <-chan *AnalysisTask, dict *dictionary.Dictionary, tracker *cardinality.Tracker, m *metrics.Metrics, extractor *Extractor) *WorkerPool {
	return &WorkerPool{
		workers:   workers,
		tasks:     tasks,
		dict:      dict,
		tracker:   tracker,
		m:         m,
		extractor: extractor,
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
	start := time.Now()

	attrs, err := wp.extractor.ExtractFromData(string(task.SignalType), task.Data)
	if err != nil {
		return
	}

	now := task.Timestamp
	entries := ToDictionaryEntries(attrs, now)

	for i, entry := range entries {
		change := wp.dict.Upsert(entry)
		if wp.m != nil {
			switch change {
			case dictionary.ChangeAdded:
				wp.m.DictionaryAttributesAdded.Inc()
			case dictionary.ChangeTypeModified:
				wp.m.DictionaryAttributesChanged.Inc()
			}
		}

		if wp.tracker != nil && i < len(attrs) {
			attr := attrs[i]
			if attr.Value != "" {
				wp.tracker.TrackValue(attr.Name, attr.Value)
			}
		}
	}

	if wp.m != nil {
		wp.m.PipelineProcessingTime.WithLabelValues("analysis").Observe(time.Since(start).Seconds())
		if wp.m.PipelineRingBufferSize != nil {
			wp.m.PipelineRingBufferSize.Set(float64(wp.processed.Load()))
		}
	}
}
