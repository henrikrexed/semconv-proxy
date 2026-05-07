package health

import (
	"testing"
)

func TestAggregatorReady(t *testing.T) {
	a := NewAggregator(nil)
	a.Register("receiver")
	a.Register("dictionary")
	a.Register("api")

	if a.IsReady() {
		t.Error("should not be ready with unknown components")
	}

	a.Update("receiver", StatusOK)
	a.Update("dictionary", StatusOK)
	a.Update("api", StatusOK)

	if !a.IsReady() {
		t.Error("should be ready when all OK")
	}
}

func TestAggregatorFailed(t *testing.T) {
	a := NewAggregator(nil)
	a.Register("receiver")
	a.Register("dictionary")

	a.Update("receiver", StatusOK)
	a.Update("dictionary", StatusFailed)

	if a.IsAlive() {
		t.Error("should not be alive with failed component")
	}
	if a.IsReady() {
		t.Error("should not be ready with failed component")
	}
}

func TestAggregatorDegraded(t *testing.T) {
	a := NewAggregator(nil)
	a.Register("receiver")
	a.Register("dictionary")

	a.Update("receiver", StatusOK)
	a.Update("dictionary", StatusDegraded)

	if !a.IsAlive() {
		t.Error("should be alive with degraded component")
	}
	if !a.IsReady() {
		t.Error("should be ready with degraded component")
	}
}

func TestGetAll(t *testing.T) {
	a := NewAggregator(nil)
	a.Register("a")
	a.Register("b")
	a.Update("a", StatusOK)
	a.Update("b", StatusDegraded)

	all := a.GetAll()
	if len(all) != 2 {
		t.Errorf("GetAll() count = %d, want 2", len(all))
	}
}
