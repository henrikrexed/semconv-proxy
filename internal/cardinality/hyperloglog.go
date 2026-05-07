package cardinality

import (
	"math"
	"sync"
)

type HyperLogLog struct {
	mu      sync.Mutex
	reg     []uint8
	m       uint32
	p       uint8
	alphaMM float64
}

func NewHyperLogLog(p uint8) *HyperLogLog {
	m := uint32(1) << p
	return &HyperLogLog{
		reg:     make([]uint8, m),
		m:       m,
		p:       p,
		alphaMM: getAlphaMM(m),
	}
}

func (h *HyperLogLog) Add(hash uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	idx := uint32(hash & (uint64(h.m) - 1))
	w := hash >> h.p
	rho := uint8(1)
	for (w&1) == 0 && rho <= 64-h.p {
		rho++
		w >>= 1
	}
	if rho > h.reg[idx] {
		h.reg[idx] = rho
	}
}

func (h *HyperLogLog) Count() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	sum := 0.0
	zeroCount := 0
	for i := uint32(0); i < h.m; i++ {
		if h.reg[i] == 0 {
			zeroCount++
		}
		sum += math.Pow(2.0, -float64(h.reg[i]))
	}
	estimate := h.alphaMM / sum
	if estimate <= 2.5*float64(h.m) && zeroCount > 0 {
		estimate = float64(h.m) * math.Log(float64(h.m)/float64(zeroCount))
	}
	return uint64(estimate + 0.5)
}

func (h *HyperLogLog) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.reg {
		h.reg[i] = 0
	}
}

func getAlphaMM(m uint32) float64 {
	switch m {
	case 16:
		return 0.673 * float64(m) * float64(m)
	case 32:
		return 0.697 * float64(m) * float64(m)
	case 64:
		return 0.709 * float64(m) * float64(m)
	default:
		return (0.7213 / (1.0 + 1.079/float64(m))) * float64(m) * float64(m)
	}
}
