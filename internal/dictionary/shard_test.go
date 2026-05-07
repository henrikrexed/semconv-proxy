package dictionary

import (
	"fmt"
	"testing"
)

func TestShardUpsertAndGet(t *testing.T) {
	s := newShard(100)
	entry := &AttributeEntry{
		Name:   "test",
		Type:   "string",
		Status: StatusActive,
	}
	s.upsert(entry)
	got, ok := s.get("test")
	if !ok {
		t.Fatal("get returned false")
	}
	if got.Name != "test" {
		t.Errorf("got.Name = %q, want %q", got.Name, "test")
	}
}

func TestShardCount(t *testing.T) {
	s := newShard(100)
	if s.count() != 0 {
		t.Errorf("initial count = %d, want 0", s.count())
	}
	s.upsert(&AttributeEntry{Name: "a", Status: StatusActive})
	s.upsert(&AttributeEntry{Name: "b", Status: StatusActive})
	if s.count() != 2 {
		t.Errorf("count = %d, want 2", s.count())
	}
}

func TestShardDelete(t *testing.T) {
	s := newShard(100)
	s.upsert(&AttributeEntry{Name: "del", Status: StatusActive})
	if !s.delete("del") {
		t.Error("delete returned false")
	}
	if s.delete("nonexistent") {
		t.Error("delete nonexistent returned true")
	}
}

func TestMergeSignalTypes(t *testing.T) {
	tests := []struct {
		name     string
		existing []SignalType
		incoming []SignalType
		want     int
	}{
		{"no overlap", []SignalType{SignalTypeMetric}, []SignalType{SignalTypeTrace}, 2},
		{"full overlap", []SignalType{SignalTypeMetric}, []SignalType{SignalTypeMetric}, 1},
		{"partial", []SignalType{SignalTypeMetric, SignalTypeTrace}, []SignalType{SignalTypeTrace, SignalTypeLog}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeSignalTypes(tt.existing, tt.incoming)
			if len(result) != tt.want {
				t.Errorf("result count = %d, want %d", len(result), tt.want)
			}
		})
	}
}

func BenchmarkShardRead(b *testing.B) {
	s := newShard(1000)
	for i := 0; i < 1000; i++ {
		s.upsert(&AttributeEntry{Name: fmt.Sprintf("attr.%d", i), Status: StatusActive})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.get(fmt.Sprintf("attr.%d", i%1000))
	}
}
