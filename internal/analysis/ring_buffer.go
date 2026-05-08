package analysis

import (
	"sync"
	"sync/atomic"
	"time"
)

type SignalType string

const (
	SignalMetric SignalType = "metric"
	SignalTrace  SignalType = "trace"
	SignalLog    SignalType = "log"
)

type AnalysisTask struct {
	SignalType SignalType `json:"signal_type"`
	Timestamp  time.Time  `json:"timestamp"`
	Data       []byte     `json:"data"`
}

type RingBuffer struct {
	mu       sync.Mutex
	buf      []*AnalysisTask
	capacity int
	head     int
	tail     int
	full     bool
	count    atomic.Int64
	dropped  atomic.Int64
	writeCh  chan *AnalysisTask
}

func NewRingBuffer(capacity int) *RingBuffer {
	rb := &RingBuffer{
		buf:      make([]*AnalysisTask, capacity),
		capacity: capacity,
		head:     0,
		tail:     0,
		full:     false,
		writeCh:  make(chan *AnalysisTask, capacity),
	}
	return rb
}

func (rb *RingBuffer) Write(task *AnalysisTask) {
	rb.mu.Lock()
	if rb.full {
		rb.dropped.Add(1)
		rb.head = (rb.head + 1) % rb.capacity
	}
	rb.buf[rb.tail] = task
	rb.tail = (rb.tail + 1) % rb.capacity
	if rb.tail == rb.head {
		rb.full = true
	}
	rb.mu.Unlock()

	rb.count.Add(1)

	select {
	case rb.writeCh <- task:
	default:
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
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.full {
		return rb.capacity
	}
	if rb.tail >= rb.head {
		return rb.tail - rb.head
	}
	return rb.capacity - rb.head + rb.tail
}
