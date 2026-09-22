package helpers

import (
	"sort"
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/reverse"
)

func FillSymbol(fill models.Fill) string {
	return strings.ToUpper(strings.TrimSpace(fill.Symbol))
}

func FillDelta(fill models.Fill) float64 {
	return SideSign(fill.Side) * fill.Size.Float64()
}

func SeedFromOpenPositions(positions []models.OpenPosition) map[string]float64 {
	out := make(map[string]float64, len(positions))
	for _, pos := range positions {
		symbol := strings.ToUpper(strings.TrimSpace(pos.Symbol))
		if symbol == "" {
			continue
		}
		out[symbol] += positionSideSign(pos.Side) * pos.Size.Float64()
	}
	return out
}

func positionSideSign(side string) float64 {
	if strings.EqualFold(strings.TrimSpace(side), "short") {
		return -1
	}
	return 1
}

type FillSegmenter struct {
	walker *reverse.Walker[models.Fill]
}

// NewFillSegmenter seeds the walker with the live open positions up front, so
// Resolved() stays false until each of them has been walked back to zero,
// even for a symbol that has not produced a fill yet.
func NewFillSegmenter(openPositions []models.OpenPosition) *FillSegmenter {
	seeds := SeedFromOpenPositions(openPositions)
	walker := reverse.NewWalker[models.Fill](reverse.SeedFromMap(seeds))
	for symbol, size := range seeds {
		walker.Seed(symbol, size)
	}
	return &FillSegmenter{walker: walker}
}

func (s *FillSegmenter) PushOlderBatch(fills []models.Fill) [][]models.Fill {
	batch := make([]models.Fill, len(fills))
	copy(batch, fills)
	sortFillsDescending(batch)

	groups := make([][]models.Fill, 0)
	for _, fill := range batch {
		size := fill.Size.Float64()
		if size <= 0 {
			continue
		}
		if group, ok := s.walker.Push(FillSymbol(fill), FillDelta(fill), fill); ok {
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

// OpenFills returns, per symbol and oldest first, the fills that belong to
// the seeded open positions.
func (s *FillSegmenter) OpenFills() map[string][]models.Fill {
	return s.walker.Pending()
}

func sortFillsDescending(fills []models.Fill) {
	sort.SliceStable(fills, func(i, j int) bool {
		ti, _ := ParseTime(fills[i].FillTime)
		tj, _ := ParseTime(fills[j].FillTime)
		if ti.Equal(tj) {
			return fills[i].FillID > fills[j].FillID
		}
		return ti.After(tj)
	})
}
