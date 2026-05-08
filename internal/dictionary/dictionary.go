package dictionary

import (
	"fmt"
	"hash/fnv"
	"log/slog"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

type ChangeType int

const (
	ChangeNone ChangeType = iota
	ChangeAdded
	ChangeTypeModified
	ChangeRemoved
)

type Config struct {
	ShardCount   int
	GlobalBudget int
	PerAttrCap   int
}

type Dictionary struct {
	shards     []*shard
	config     *Config
	logger     *slog.Logger
	totalCount atomic.Int64
}

func New(cfg *Config, logger *slog.Logger) (*Dictionary, error) {
	if cfg.ShardCount <= 0 {
		return nil, fmt.Errorf("dictionary: shard count must be positive, got %d", cfg.ShardCount)
	}
	d := &Dictionary{
		shards: make([]*shard, cfg.ShardCount),
		config: cfg,
		logger: logger,
	}
	for i := range d.shards {
		d.shards[i] = newShard(cfg.PerAttrCap)
	}
	return d, nil
}

func (d *Dictionary) shardIndex(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32() % uint32(len(d.shards))
}

func (d *Dictionary) Upsert(entry *AttributeEntry) ChangeType {
	idx := d.shardIndex(entry.Name)
	s := d.shards[idx]

	existing, exists := s.get(entry.Name)
	if !exists && d.config.GlobalBudget > 0 && int(d.totalCount.Load()) >= d.config.GlobalBudget {
		return ChangeNone
	}

	change := s.upsert(entry)
	if change == ChangeAdded && !exists {
		d.totalCount.Add(1)
	}
	_ = existing
	return change
}

func (d *Dictionary) Get(name string) (*AttributeEntry, bool) {
	idx := d.shardIndex(name)
	return d.shards[idx].get(name)
}

func (d *Dictionary) Delete(name string) bool {
	idx := d.shardIndex(name)
	deleted := d.shards[idx].delete(name)
	if deleted {
		d.totalCount.Add(-1)
	}
	return deleted
}

func (d *Dictionary) List(filter *Filter) []*AttributeEntry {
	var all []*AttributeEntry
	for _, s := range d.shards {
		all = append(all, s.list()...)
	}
	if filter != nil {
		all = applyFilter(all, filter)
	}
	return all
}

func (d *Dictionary) Count() int64 {
	return d.totalCount.Load()
}

func (d *Dictionary) MarkStale(now time.Time, staleThreshold time.Duration) int {
	count := 0
	for _, s := range d.shards {
		s.mu.Lock()
		for _, e := range s.entries {
			if e.Status == StatusActive && now.Sub(e.LastSeen) > staleThreshold {
				e.Status = StatusStale
				count++
			}
		}
		s.mu.Unlock()
	}
	return count
}

func (d *Dictionary) PurgeExpired(now time.Time, purgeThreshold time.Duration) int {
	count := 0
	for _, s := range d.shards {
		s.mu.Lock()
		for name, e := range s.entries {
			if now.Sub(e.LastSeen) > purgeThreshold {
				delete(s.entries, name)
				d.totalCount.Add(-1)
				count++
			}
		}
		s.mu.Unlock()
	}
	return count
}

type Filter struct {
	SignalType SignalType
	Pattern    string
	Sort       string
	Order      string
	Limit      int
	Offset     int
}

func applyFilter(entries []*AttributeEntry, f *Filter) []*AttributeEntry {
	filtered := entries
	if f.SignalType != "" {
		filtered = filterBySignalType(filtered, f.SignalType)
	}
	if f.Pattern != "" {
		filtered = filterByPattern(filtered, f.Pattern)
	}
	sortEntries(filtered, f.Sort, f.Order)
	if f.Offset > 0 {
		if f.Offset >= len(filtered) {
			return []*AttributeEntry{}
		}
		filtered = filtered[f.Offset:]
	}
	if f.Limit > 0 && f.Limit < len(filtered) {
		filtered = filtered[:f.Limit]
	}
	return filtered
}

func filterBySignalType(entries []*AttributeEntry, st SignalType) []*AttributeEntry {
	var result []*AttributeEntry
	for _, e := range entries {
		for _, t := range e.SignalTypes {
			if t == st {
				result = append(result, e)
				break
			}
		}
	}
	return result
}

func filterByPattern(entries []*AttributeEntry, pattern string) []*AttributeEntry {
	var result []*AttributeEntry
	for _, e := range entries {
		if matchPattern(e.Name, pattern) {
			result = append(result, e)
		}
	}
	return result
}

func matchPattern(name, pattern string) bool {
	if !strings.Contains(pattern, "*") {
		return strings.Contains(name, pattern)
	}
	parts := strings.Split(pattern, "*")
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(name, part)
		if idx < 0 {
			return false
		}
		if i == 0 && idx != 0 {
			return false
		}
		name = name[idx+len(part):]
	}
	return true
}

func sortEntries(entries []*AttributeEntry, sortField, order string) {
	sort.Slice(entries, func(i, j int) bool {
		var less bool
		switch sortField {
		case "cardinality":
			less = entries[i].Cardinality < entries[j].Cardinality
		case "first_seen":
			less = entries[i].FirstSeen.Before(entries[j].FirstSeen)
		case "last_seen":
			less = entries[i].LastSeen.Before(entries[j].LastSeen)
		default:
			less = entries[i].Name < entries[j].Name
		}
		if order == "desc" {
			return !less
		}
		return less
	})
}
