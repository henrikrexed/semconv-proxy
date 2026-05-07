package cardinality

import (
	"math"
)

type CountMinSketch struct {
	depth uint
	width uint
	table [][]uint64
}

func NewCountMinSketch(epsilon, delta float64) *CountMinSketch {
	width := uint(math.Ceil(math.E / epsilon))
	depth := uint(math.Ceil(math.Log(1.0 / delta)))
	table := make([][]uint64, depth)
	for i := range table {
		table[i] = make([]uint64, width)
	}
	return &CountMinSketch{depth: depth, width: width, table: table}
}

func (c *CountMinSketch) Add(item string, count uint64) {
	for i := uint(0); i < c.depth; i++ {
		idx := uint(c.hash(item, i)) % c.width
		c.table[i][idx] += count
	}
}

func (c *CountMinSketch) Count(item string) uint64 {
	minCount := uint64(math.MaxUint64)
	for i := uint(0); i < c.depth; i++ {
		idx := uint(c.hash(item, i)) % c.width
		if c.table[i][idx] < minCount {
			minCount = c.table[i][idx]
		}
	}
	return minCount
}

func (c *CountMinSketch) Reset() {
	for i := range c.table {
		for j := range c.table[i] {
			c.table[i][j] = 0
		}
	}
}

func (c *CountMinSketch) hash(item string, row uint) uint64 {
	h := fnvHash64(item)
	h ^= uint64(row) * 0x9e3779b97f4a7c15
	h = (h ^ (h >> 30)) * 0xbf58476d1ce4e5b9
	h = (h ^ (h >> 27)) * 0x94d049bb133111eb
	h ^= h >> 31
	return h
}

func fnvHash64(s string) uint64 {
	h := uint64(14695981039346656037)
	for _, c := range s {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return h
}
