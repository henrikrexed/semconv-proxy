package lifecycle

import (
	"context"
	"testing"
	"time"

	"log/slog"
)

type mockComponent struct {
	started bool
	stopped bool
	startFn func(ctx context.Context) error
	stopFn  func(ctx context.Context) error
}

func (m *mockComponent) Start(ctx context.Context) error {
	m.started = true
	if m.startFn != nil {
		return m.startFn(ctx)
	}
	return nil
}

func (m *mockComponent) Stop(ctx context.Context) error {
	m.stopped = true
	if m.stopFn != nil {
		return m.stopFn(ctx)
	}
	return nil
}

func TestCoordinatorStartStopAll(t *testing.T) {
	c := NewCoordinator(slog.Default(), 5*time.Second)

	comp1 := &mockComponent{}
	comp2 := &mockComponent{}

	c.Add(comp1)
	c.Add(comp2)

	ctx := context.Background()
	if err := c.StartAll(ctx); err != nil {
		t.Fatalf("StartAll error: %v", err)
	}

	if !comp1.started || !comp2.started {
		t.Error("expected all components started")
	}

	c.StopAll(ctx)

	if !comp1.stopped || !comp2.stopped {
		t.Error("expected all components stopped")
	}
}

func TestCoordinatorStopReversesOrder(t *testing.T) {
	c := NewCoordinator(slog.Default(), 5*time.Second)
	var stopOrder []int

	comp1 := &mockComponent{stopFn: func(ctx context.Context) error { stopOrder = append(stopOrder, 1); return nil }}
	comp2 := &mockComponent{stopFn: func(ctx context.Context) error { stopOrder = append(stopOrder, 2); return nil }}

	c.Add(comp1)
	c.Add(comp2)

	ctx := context.Background()
	c.StartAll(ctx)
	c.StopAll(ctx)

	if len(stopOrder) != 2 {
		t.Fatalf("stopOrder len = %d, want 2", len(stopOrder))
	}
	if stopOrder[0] != 2 || stopOrder[1] != 1 {
		t.Errorf("stopOrder = %v, want [2, 1]", stopOrder)
	}
}

func TestCoordinatorStartError(t *testing.T) {
	c := NewCoordinator(slog.Default(), 5*time.Second)

	comp1 := &mockComponent{}
	comp2 := &mockComponent{startFn: func(ctx context.Context) error { return context.Canceled }}

	c.Add(comp1)
	c.Add(comp2)

	err := c.StartAll(context.Background())
	if err == nil {
		t.Fatal("expected error from StartAll")
	}
}
