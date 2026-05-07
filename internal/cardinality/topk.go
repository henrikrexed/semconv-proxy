package cardinality

import (
	"container/heap"
	"sort"
)

type TopKItem struct {
	Value string
	Count uint64
}

type topKHeap []TopKItem

func (h topKHeap) Len() int           { return len(h) }
func (h topKHeap) Less(i, j int) bool { return h[i].Count < h[j].Count }
func (h topKHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *topKHeap) Push(x interface{}) {
	v, _ := x.(TopKItem)
	*h = append(*h, v)
}
func (h *topKHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

type TopK struct {
	k    int
	cms  *CountMinSketch
	seen map[string]bool
}

func NewTopK(k int, epsilon, delta float64) *TopK {
	return &TopK{
		k:    k,
		cms:  NewCountMinSketch(epsilon, delta),
		seen: make(map[string]bool),
	}
}

func (t *TopK) Add(value string) {
	t.cms.Add(value, 1)
	t.seen[value] = true
}

func (t *TopK) Top() []TopKItem {
	h := &topKHeap{}
	heap.Init(h)
	for value := range t.seen {
		count := t.cms.Count(value)
		if h.Len() < t.k {
			heap.Push(h, TopKItem{Value: value, Count: count})
		} else if count > (*h)[0].Count {
			heap.Pop(h)
			heap.Push(h, TopKItem{Value: value, Count: count})
		}
	}
	result := make([]TopKItem, h.Len())
	for i := h.Len() - 1; i >= 0; i-- {
		item, _ := heap.Pop(h).(TopKItem)
		result[i] = item
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})
	return result
}

func (t *TopK) Reset() {
	t.cms.Reset()
	t.seen = make(map[string]bool)
}
