package health

import (
	"log/slog"
	"sync"
)

type ComponentStatus int

const (
	StatusUnknown ComponentStatus = iota
	StatusOK
	StatusDegraded
	StatusFailed
)

func (s ComponentStatus) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusDegraded:
		return "degraded"
	case StatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

type ComponentHealth struct {
	Name   string
	Status ComponentStatus
}

type Aggregator struct {
	mu         sync.RWMutex
	components map[string]ComponentStatus
	logger     *slog.Logger
}

func NewAggregator(logger *slog.Logger) *Aggregator {
	return &Aggregator{
		components: make(map[string]ComponentStatus),
		logger:     logger,
	}
}

func (a *Aggregator) Register(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.components[name] = StatusUnknown
}

func (a *Aggregator) Update(name string, status ComponentStatus) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.components[name] = status
}

func (a *Aggregator) IsReady() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, s := range a.components {
		if s == StatusUnknown || s == StatusFailed {
			return false
		}
	}
	return true
}

func (a *Aggregator) IsAlive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, s := range a.components {
		if s == StatusFailed {
			return false
		}
	}
	return true
}

func (a *Aggregator) GetAll() []ComponentHealth {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var result []ComponentHealth
	for name, status := range a.components {
		result = append(result, ComponentHealth{Name: name, Status: status})
	}
	return result
}
