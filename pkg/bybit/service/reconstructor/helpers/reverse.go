package helpers

import (
	"math"
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/reverse"
)

func SeedFromOpenPositions(positions []models.Position) map[string]float64 {
	out := make(map[string]float64, len(positions))
	for _, pos := range positions {
		size := math.Abs(MustFloat(pos.Size))
		if size == 0 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(pos.Side), "sell") {
			size = -size
		}
		out[PositionKey(pos.Symbol, int(pos.PositionIdx.Int64()))] += size
	}
	return out
}

type FillSegmenter struct {
	walker *reverse.Walker[Fill]
}

// NewFillSegmenter seeds the walker with the live open positions up front, so
// Resolved() stays false until each of them has been walked back to zero.
func NewFillSegmenter(openPositions []models.Position) *FillSegmenter {
	seeds := SeedFromOpenPositions(openPositions)
	walker := reverse.NewWalker[Fill](reverse.SeedFromMap(seeds))
	for key, size := range seeds {
		walker.Seed(key, size)
	}
	return &FillSegmenter{walker: walker}
}

func (s *FillSegmenter) PushOlderBatch(fills []Fill) [][]Fill {
	batch := make([]Fill, len(fills))
	copy(batch, fills)
	SortFillsDesc(batch)

	groups := make([][]Fill, 0)
	for _, fill := range batch {
		delta := fill.Delta()
		if delta == 0 {
			continue
		}
		if group, ok := s.walker.Push(fill.Key(), delta, fill); ok {
			groups = append(groups, group.Fills)
		}
	}
	return groups
}

func (s *FillSegmenter) Flat() bool {
	return s.walker.Flat()
}

// Resolved reports whether every seeded open position has been walked back
// to its opening fill.
func (s *FillSegmenter) Resolved() bool {
	return s.walker.Resolved()
}
