package cardinality

import (
	"log/slog"
	"sync"
	"sync/atomic"
)

type AttrTracker struct {
	mu         sync.RWMutex
	exactCount map[string]struct{}
	hll        *HyperLogLog
	topK       *TopK
	cap        int
	exact      bool
	totalCount atomic.Int64
}

type Tracker struct {
	mu           sync.RWMutex
	attrs        map[string]*AttrTracker
	globalBudget int
	perAttrCap   int
	totalEntries atomic.Int64
	logger       *slog.Logger
}

func NewTracker(globalBudget, perAttrCap int, logger *slog.Logger) *Tracker {
	return &Tracker{
		attrs:        make(map[string]*AttrTracker),
		globalBudget: globalBudget,
		perAttrCap:   perAttrCap,
		logger:       logger,
	}
}

func (t *Tracker) TrackValue(attrName, value string) {
	t.totalEntries.Add(1)
	t.mu.Lock()
	tracker, ok := t.attrs[attrName]
	if !ok {
		if int(t.totalEntries.Load()) > t.globalBudget {
			t.mu.Unlock()
			return
		}
		tracker = &AttrTracker{
			exactCount: make(map[string]struct{}),
			hll:        NewHyperLogLog(14),
			topK:       NewTopK(50, 0.001, 0.01),
			cap:        t.perAttrCap,
			exact:      true,
		}
		t.attrs[attrName] = tracker
	}
	t.mu.Unlock()

	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	tracker.totalCount.Add(1)
	tracker.topK.Add(value)

	if tracker.exact {
		tracker.exactCount[value] = struct{}{}
		if len(tracker.exactCount) > tracker.cap {
			tracker.exact = false
		}
	} else {
		tracker.hll.Add(fnvHash(value))
	}
}

func (t *Tracker) Cardinality(attrName string) int64 {
	t.mu.RLock()
	tracker, ok := t.attrs[attrName]
	t.mu.RUnlock()
	if !ok {
		return 0
	}
	tracker.mu.RLock()
	defer tracker.mu.RUnlock()
	if tracker.exact {
		return int64(len(tracker.exactCount))
	}
	return int64(tracker.hll.Count())
}

func (t *Tracker) TopValues(attrName string) []TopKItem {
	t.mu.RLock()
	tracker, ok := t.attrs[attrName]
	t.mu.RUnlock()
	if !ok {
		return nil
	}
	return tracker.topK.Top()
}

func (t *Tracker) GlobalUtilization() (used int, limit int, pct float64) {
	t.mu.RLock()
	count := len(t.attrs)
	t.mu.RUnlock()
	return count, t.globalBudget, float64(count) / float64(t.globalBudget) * 100
}

func (t *Tracker) HighCardinality(threshold int64) []AttrCardinality {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var result []AttrCardinality
	for name, tracker := range t.attrs {
		c := tracker.Cardinality()
		if c >= threshold {
			result = append(result, AttrCardinality{
				Name:        name,
				Cardinality: c,
				Cap:         t.perAttrCap,
			})
		}
	}
	return result
}

func (at *AttrTracker) Cardinality() int64 {
	at.mu.RLock()
	defer at.mu.RUnlock()
	if at.exact {
		return int64(len(at.exactCount))
	}
	return int64(at.hll.Count())
}

type AttrCardinality struct {
	Name        string `json:"name"`
	Cardinality int64  `json:"cardinality"`
	Cap         int    `json:"cap"`
}

func fnvHash(s string) uint64 {
	h := uint64(2166136261)
	for _, c := range s {
		h ^= uint64(c)
		h *= 16777619
	}
	return h
}
