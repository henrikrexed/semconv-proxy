package cardinality

import (
	"fmt"
	"testing"
)

func TestHyperLogLog(t *testing.T) {
	hll := NewHyperLogLog(14)
	for i := 0; i < 10000; i++ {
		h := fnvHashItem(i)
		hll.Add(h)
	}
	count := hll.Count()
	errorPct := float64(int64(count)-10000) / 10000.0 * 100
	if errorPct < -15 || errorPct > 15 {
		t.Errorf("HLL count = %d, expected ~10000, error%%=%.1f%%", count, errorPct)
	}
}

func TestHyperLogLog_Unique(t *testing.T) {
	hll := NewHyperLogLog(14)
	hll.Add(fnvHashItem(1))
	hll.Add(fnvHashItem(2))
	hll.Add(fnvHashItem(3))
	count := hll.Count()
	if count < 2 || count > 5 {
		t.Errorf("HLL count for 3 unique = %d, expected ~3", count)
	}
}

func TestCountMinSketch(t *testing.T) {
	cms := NewCountMinSketch(0.001, 0.01)
	cms.Add("foo", 1)
	cms.Add("foo", 1)
	cms.Add("foo", 1)
	cms.Add("bar", 1)

	if c := cms.Count("foo"); c < 3 {
		t.Errorf("CMS count for foo = %d, expected >= 3", c)
	}
	if c := cms.Count("bar"); c < 1 {
		t.Errorf("CMS count for bar = %d, expected >= 1", c)
	}
	if c := cms.Count("baz"); c != 0 {
		t.Errorf("CMS count for baz = %d, expected 0", c)
	}
}

func TestTopK(t *testing.T) {
	tk := NewTopK(3, 0.001, 0.01)
	tk.Add("a")
	tk.Add("b")
	tk.Add("b")
	tk.Add("c")
	tk.Add("c")
	tk.Add("c")

	top := tk.Top()
	if len(top) != 3 {
		t.Fatalf("TopK len = %d, want 3", len(top))
	}
	if top[0].Value != "c" {
		t.Errorf("TopK[0] = %q, want %q", top[0].Value, "c")
	}
}

func TestTracker(t *testing.T) {
	tracker := NewTracker(100, 10, nil)
	for i := 0; i < 3; i++ {
		tracker.TrackValue("attr1", fmt.Sprintf("val%d", i))
	}
	tracker.TrackValue("attr2", "val1")

	c := tracker.Cardinality("attr1")
	if c != 3 {
		t.Errorf("Cardinality(attr1) = %d, want 3", c)
	}
	c = tracker.Cardinality("attr2")
	if c != 1 {
		t.Errorf("Cardinality(attr2) = %d, want 1", c)
	}

	used, limit, pct := tracker.GlobalUtilization()
	if used != 2 {
		t.Errorf("GlobalUtilization used = %d, want 2", used)
	}
	if limit != 100 {
		t.Errorf("GlobalUtilization limit = %d, want 100", limit)
	}
	if pct < 1 || pct > 5 {
		t.Errorf("GlobalUtilization pct = %.1f, want ~2", pct)
	}
}

func TestTrackerHighCardinality(t *testing.T) {
	tracker := NewTracker(1000, 100, nil)
	for i := 0; i < 50; i++ {
		tracker.TrackValue("high_card", fmt.Sprintf("val%d", i))
	}
	for i := 0; i < 3; i++ {
		tracker.TrackValue("low_card", fmt.Sprintf("val%d", i))
	}

	high := tracker.HighCardinality(10)
	if len(high) != 1 {
		t.Errorf("HighCardinality(10) count = %d, want 1", len(high))
	}
	if len(high) > 0 && high[0].Name != "high_card" {
		t.Errorf("HighCardinality[0].Name = %q, want %q", high[0].Name, "high_card")
	}
}

func fnvHashItem(i int) uint64 {
	h := uint64(2166136261)
	h ^= uint64(i)
	h *= 16777619
	h ^= uint64(i >> 8)
	h *= 16777619
	h ^= uint64(i >> 16)
	h *= 16777619
	return h
}
