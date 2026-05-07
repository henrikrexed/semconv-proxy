package dictionary

import "sync"

type shard struct {
	mu      sync.RWMutex
	entries map[string]*AttributeEntry
	cap     int
}

func newShard(perAttrCap int) *shard {
	return &shard{
		entries: make(map[string]*AttributeEntry),
		cap:     perAttrCap,
	}
}

func (s *shard) get(name string) (*AttributeEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[name]
	return e, ok
}

func (s *shard) upsert(entry *AttributeEntry) ChangeType {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.entries[entry.Name]
	if !ok {
		s.entries[entry.Name] = entry
		return ChangeAdded
	}
	change := ChangeNone
	if existing.Type != entry.Type {
		change = ChangeTypeModified
	}
	existing.LastSeen = entry.LastSeen
	if len(entry.SignalTypes) > 0 {
		existing.SignalTypes = mergeSignalTypes(existing.SignalTypes, entry.SignalTypes)
	}
	if entry.Type != "" {
		existing.Type = entry.Type
	}
	existing.Status = StatusActive
	return change
}

func (s *shard) delete(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.entries[name]
	if ok {
		delete(s.entries, name)
	}
	return ok
}

func (s *shard) list() []*AttributeEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*AttributeEntry, 0, len(s.entries))
	for _, e := range s.entries {
		result = append(result, e)
	}
	return result
}

func (s *shard) count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

func mergeSignalTypes(existing, incoming []SignalType) []SignalType {
	seen := make(map[SignalType]bool)
	for _, st := range existing {
		seen[st] = true
	}
	for _, st := range incoming {
		if !seen[st] {
			existing = append(existing, st)
			seen[st] = true
		}
	}
	return existing
}
