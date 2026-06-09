package semconv

import (
	"sort"
	"strings"
)

// Query describes a faceted search over the registry. Zero-value fields are
// ignored. Text matches as a case-insensitive substring of name or brief.
type Query struct {
	Text      string
	Types     []ItemType
	Stability []string
	Namespace string
	Limit     int
	Offset    int
}

// FacetCount is a single facet value and the number of matching items.
type FacetCount struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// Facets summarizes the distribution of the text-matched set across each facet
// dimension, before the specific facet selections are applied. This lets the UI
// show "how many results would each facet value yield".
type Facets struct {
	Type      []FacetCount `json:"type"`
	Stability []FacetCount `json:"stability"`
	Namespace []FacetCount `json:"namespace"`
}

// Result is a page of search results plus total count and facet summary.
type Result struct {
	Total  int    `json:"total"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
	Items  []Item `json:"items"`
	Facets Facets `json:"facets"`
}

// Items returns the full, sorted item slice (read-only; callers must not mutate).
func (r *Registry) Items() []Item { return r.items }

// Len reports the number of indexed items.
func (r *Registry) Len() int { return len(r.items) }

// URL reports the source registry URL recorded in the snapshot.
func (r *Registry) URL() string { return r.url }

// Get returns the item with the given key (e.g. "attribute:http.request.method").
func (r *Registry) Get(key string) (Item, bool) {
	for _, it := range r.items {
		if it.Key == key {
			return it, true
		}
	}
	return Item{}, false
}

// Search applies the query and returns a paginated result with facet counts.
func (r *Registry) Search(q Query) Result {
	text := strings.ToLower(strings.TrimSpace(q.Text))

	// First pass: text-only match. Facets are computed over this set so the
	// counts reflect what each facet selection would narrow to.
	textMatched := make([]Item, 0, len(r.items))
	for _, it := range r.items {
		if matchesText(it, text) {
			textMatched = append(textMatched, it)
		}
	}

	facets := computeFacets(textMatched)

	// Second pass: apply facet selections.
	typeSet := toSet(itemTypeStrings(q.Types))
	stabSet := toSet(q.Stability)
	filtered := make([]Item, 0, len(textMatched))
	for _, it := range textMatched {
		if len(typeSet) > 0 && !typeSet[string(it.Type)] {
			continue
		}
		if len(stabSet) > 0 && !stabSet[it.Stability] {
			continue
		}
		if q.Namespace != "" && it.Namespace != q.Namespace {
			continue
		}
		filtered = append(filtered, it)
	}

	total := len(filtered)
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return Result{
		Total:  total,
		Offset: offset,
		Limit:  limit,
		Items:  filtered[offset:end],
		Facets: facets,
	}
}

func matchesText(it Item, text string) bool {
	if text == "" {
		return true
	}
	return strings.Contains(strings.ToLower(it.Name), text) ||
		strings.Contains(strings.ToLower(it.Brief), text)
}

func computeFacets(items []Item) Facets {
	typeCounts := map[string]int{}
	stabCounts := map[string]int{}
	nsCounts := map[string]int{}
	for _, it := range items {
		typeCounts[string(it.Type)]++
		if it.Stability != "" {
			stabCounts[it.Stability]++
		}
		if it.Namespace != "" {
			nsCounts[it.Namespace]++
		}
	}
	return Facets{
		Type:      sortedFacets(typeCounts),
		Stability: sortedFacets(stabCounts),
		Namespace: sortedFacets(nsCounts),
	}
}

// sortedFacets returns facet counts ordered by descending count, then value.
func sortedFacets(m map[string]int) []FacetCount {
	out := make([]FacetCount, 0, len(m))
	for v, c := range m {
		out = append(out, FacetCount{Value: v, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})
	return out
}

func itemTypeStrings(types []ItemType) []string {
	out := make([]string, len(types))
	for i, t := range types {
		out[i] = string(t)
	}
	return out
}

func toSet(vals []string) map[string]bool {
	if len(vals) == 0 {
		return nil
	}
	set := make(map[string]bool, len(vals))
	for _, v := range vals {
		if v != "" {
			set[v] = true
		}
	}
	return set
}
