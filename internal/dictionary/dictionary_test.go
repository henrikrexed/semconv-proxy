package dictionary

import (
	"fmt"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		shardCnt int
		wantErr  bool
	}{
		{name: "valid", shardCnt: 64, wantErr: false},
		{name: "single shard", shardCnt: 1, wantErr: false},
		{name: "zero shards", shardCnt: 0, wantErr: true},
		{name: "negative shards", shardCnt: -1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := New(&Config{ShardCount: tt.shardCnt, GlobalBudget: 1000, PerAttrCap: 100}, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && d == nil {
				t.Error("New() returned nil without error")
			}
		})
	}
}

func TestUpsertAndGet(t *testing.T) {
	d, _ := New(&Config{ShardCount: 64, GlobalBudget: 1000, PerAttrCap: 100}, nil)
	now := time.Now()

	entry := &AttributeEntry{
		Name:        "http.request.method",
		Type:        "string",
		SignalTypes: []SignalType{SignalTypeMetric},
		FirstSeen:   now,
		LastSeen:    now,
		Status:      StatusActive,
	}

	change := d.Upsert(entry)
	if change != ChangeAdded {
		t.Errorf("first Upsert change = %v, want ChangeAdded", change)
	}

	got, ok := d.Get("http.request.method")
	if !ok {
		t.Fatal("Get() returned false")
	}
	if got.Name != "http.request.method" {
		t.Errorf("Get().Name = %q, want %q", got.Name, "http.request.method")
	}
	if got.Type != "string" {
		t.Errorf("Get().Type = %q, want %q", got.Type, "string")
	}
}

func TestUpsertUpdate(t *testing.T) {
	d, _ := New(&Config{ShardCount: 64, GlobalBudget: 1000, PerAttrCap: 100}, nil)
	now := time.Now()

	d.Upsert(&AttributeEntry{
		Name:        "test.attr",
		Type:        "string",
		SignalTypes: []SignalType{SignalTypeMetric},
		FirstSeen:   now,
		LastSeen:    now,
		Status:      StatusActive,
	})

	later := now.Add(time.Hour)
	change := d.Upsert(&AttributeEntry{
		Name:        "test.attr",
		Type:        "int",
		SignalTypes: []SignalType{SignalTypeTrace},
		FirstSeen:   now,
		LastSeen:    later,
		Status:      StatusActive,
	})
	if change != ChangeTypeModified {
		t.Errorf("update Upsert change = %v, want ChangeTypeModified", change)
	}

	got, _ := d.Get("test.attr")
	if got.Type != "int" {
		t.Errorf("after update Type = %q, want %q", got.Type, "int")
	}
	if len(got.SignalTypes) != 2 {
		t.Errorf("SignalTypes count = %d, want 2", len(got.SignalTypes))
	}
}

func TestDelete(t *testing.T) {
	d, _ := New(&Config{ShardCount: 64, GlobalBudget: 1000, PerAttrCap: 100}, nil)
	now := time.Now()

	d.Upsert(&AttributeEntry{Name: "to.delete", Type: "string", FirstSeen: now, LastSeen: now, Status: StatusActive})
	deleted := d.Delete("to.delete")
	if !deleted {
		t.Error("Delete() returned false")
	}
	_, ok := d.Get("to.delete")
	if ok {
		t.Error("Get() after Delete() returned true")
	}
	deleted = d.Delete("nonexistent")
	if deleted {
		t.Error("Delete nonexistent returned true")
	}
}

func TestCount(t *testing.T) {
	d, _ := New(&Config{ShardCount: 64, GlobalBudget: 1000, PerAttrCap: 100}, nil)
	now := time.Now()

	if d.Count() != 0 {
		t.Errorf("initial Count = %d, want 0", d.Count())
	}
	d.Upsert(&AttributeEntry{Name: "a", FirstSeen: now, LastSeen: now, Status: StatusActive})
	d.Upsert(&AttributeEntry{Name: "b", FirstSeen: now, LastSeen: now, Status: StatusActive})
	if d.Count() != 2 {
		t.Errorf("Count after 2 adds = %d, want 2", d.Count())
	}
	d.Delete("a")
	if d.Count() != 1 {
		t.Errorf("Count after delete = %d, want 1", d.Count())
	}
}

func TestListWithFilter(t *testing.T) {
	d, _ := New(&Config{ShardCount: 64, GlobalBudget: 1000, PerAttrCap: 100}, nil)
	now := time.Now()

	entries := []*AttributeEntry{
		{Name: "http.request.method", Type: "string", SignalTypes: []SignalType{SignalTypeMetric, SignalTypeTrace}, FirstSeen: now, LastSeen: now, Status: StatusActive, Cardinality: 6},
		{Name: "k8s.pod.name", Type: "string", SignalTypes: []SignalType{SignalTypeTrace}, FirstSeen: now, LastSeen: now, Status: StatusActive, Cardinality: 847},
		{Name: "http.response.status_code", Type: "int", SignalTypes: []SignalType{SignalTypeMetric}, FirstSeen: now, LastSeen: now, Status: StatusActive, Cardinality: 50},
	}
	for _, e := range entries {
		d.Upsert(e)
	}

	tests := []struct {
		name   string
		filter *Filter
		want   int
	}{
		{name: "no filter", filter: nil, want: 3},
		{name: "by signal type metric", filter: &Filter{SignalType: SignalTypeMetric}, want: 2},
		{name: "by signal type trace", filter: &Filter{SignalType: SignalTypeTrace}, want: 2},
		{name: "by pattern http", filter: &Filter{Pattern: "http"}, want: 2},
		{name: "by pattern wildcard", filter: &Filter{Pattern: "http*"}, want: 2},
		{name: "sort by cardinality desc", filter: &Filter{Sort: "cardinality", Order: "desc", Limit: 1}, want: 1},
		{name: "offset beyond results", filter: &Filter{Offset: 100}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.List(tt.filter)
			if len(result) != tt.want {
				t.Errorf("List() count = %d, want %d", len(result), tt.want)
			}
		})
	}
}

func TestSortByCardinality(t *testing.T) {
	d, _ := New(&Config{ShardCount: 4, GlobalBudget: 1000, PerAttrCap: 100}, nil)
	now := time.Now()

	d.Upsert(&AttributeEntry{Name: "low", Cardinality: 1, FirstSeen: now, LastSeen: now, Status: StatusActive})
	d.Upsert(&AttributeEntry{Name: "high", Cardinality: 100, FirstSeen: now, LastSeen: now, Status: StatusActive})
	d.Upsert(&AttributeEntry{Name: "mid", Cardinality: 50, FirstSeen: now, LastSeen: now, Status: StatusActive})

	result := d.List(&Filter{Sort: "cardinality", Order: "desc"})
	if len(result) != 3 {
		t.Fatalf("List() count = %d, want 3", len(result))
	}
	if result[0].Name != "high" {
		t.Errorf("first entry = %q, want %q", result[0].Name, "high")
	}
}

func TestMarkStale(t *testing.T) {
	d, _ := New(&Config{ShardCount: 4, GlobalBudget: 1000, PerAttrCap: 100}, nil)
	now := time.Now()

	d.Upsert(&AttributeEntry{Name: "old", FirstSeen: now.Add(-48 * time.Hour), LastSeen: now.Add(-48 * time.Hour), Status: StatusActive})
	d.Upsert(&AttributeEntry{Name: "fresh", FirstSeen: now, LastSeen: now, Status: StatusActive})

	stale := d.MarkStale(now, 24*time.Hour)
	if stale != 1 {
		t.Errorf("MarkStale count = %d, want 1", stale)
	}

	old, _ := d.Get("old")
	if old.Status != StatusStale {
		t.Errorf("old entry status = %q, want %q", old.Status, StatusStale)
	}
}

func TestPurgeExpired(t *testing.T) {
	d, _ := New(&Config{ShardCount: 4, GlobalBudget: 1000, PerAttrCap: 100}, nil)
	now := time.Now()

	d.Upsert(&AttributeEntry{Name: "expired", FirstSeen: now.Add(-200 * time.Hour), LastSeen: now.Add(-200 * time.Hour), Status: StatusActive})
	d.Upsert(&AttributeEntry{Name: "active", FirstSeen: now, LastSeen: now, Status: StatusActive})

	purged, _ := d.PurgeExpired(now, 7*24*time.Hour)
	if purged != 1 {
		t.Errorf("PurgeExpired count = %d, want 1", purged)
	}

	_, ok := d.Get("expired")
	if ok {
		t.Error("expired entry still exists")
	}
}

func BenchmarkGet(b *testing.B) {
	d, _ := New(&Config{ShardCount: 64, GlobalBudget: 10000, PerAttrCap: 1000}, nil)
	now := time.Now()
	for i := 0; i < 1000; i++ {
		d.Upsert(&AttributeEntry{
			Name:      fmt.Sprintf("attr.%d", i),
			Type:      "string",
			FirstSeen: now,
			LastSeen:  now,
			Status:    StatusActive,
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Get(fmt.Sprintf("attr.%d", i%1000))
	}
}

func BenchmarkUpsert(b *testing.B) {
	d, _ := New(&Config{ShardCount: 64, GlobalBudget: 10000, PerAttrCap: 1000}, nil)
	now := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Upsert(&AttributeEntry{
			Name:      fmt.Sprintf("attr.%d", i%1000),
			Type:      "string",
			FirstSeen: now,
			LastSeen:  now,
			Status:    StatusActive,
		})
	}
}
