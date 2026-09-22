package helpers

import (
	"math"
	"sort"
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/binance/connector/binance/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/reverse"
)

func FillKey(fill models.Trade) string {
	return episodeKey(fill)
}

func FillDelta(fill models.Trade) float64 {
	return fillSign(fill) * MustFloat(fill.Qty)
}

func SeedFromOpenPositions(positions []models.PositionRisk) map[string]float64 {
	out := make(map[string]float64, len(positions))
	for _, pos := range positions {
		size := MustFloat(pos.PositionAmt)
		if size == 0 {
			continue
		}

		switch strings.ToUpper(strings.TrimSpace(pos.PositionSide)) {
		case "LONG":
			size = math.Abs(size)
		case "SHORT":
			size = -math.Abs(size)
		}

		out[positionKey(pos.Symbol, pos.PositionSide)] += size
	}
	return out
}

type FillSegmenter struct {
	walker *reverse.Walker[models.Trade]
}

// NewFillSegmenter seeds the walker with the live open positions up front, so
// Resolved() is false until every one of them has been walked back to zero,
// even for a symbol that has not produced a fill yet.
func NewFillSegmenter(openPositions []models.PositionRisk) *FillSegmenter {
	return &FillSegmenter{walker: reverse.NewSeededWalker[models.Trade](SeedFromOpenPositions(openPositions))}
}

func (s *FillSegmenter) PushOlderBatch(fills []models.Trade) [][]models.Trade {
	batch := make([]models.Trade, len(fills))
	copy(batch, fills)
	sort.SliceStable(batch, func(i, j int) bool {
		if batch[i].Time == batch[j].Time {
			return batch[i].ID > batch[j].ID
		}
		return batch[i].Time > batch[j].Time
	})

	groups := make([][]models.Trade, 0)
	for _, fill := range batch {
		delta := FillDelta(fill)
		if delta == 0 {
			continue
		}
		if group, ok := s.walker.Push(FillKey(fill), delta, fill); ok {
			groups = append(groups, group.Fills)
		}
	}
	return groups
}

func (s *FillSegmenter) Flat() bool {
	return s.walker.Flat()
}

// Resolved reports whether every seeded open position has been walked back to
// its opening fill.
func (s *FillSegmenter) Resolved() bool {
	return s.walker.Resolved()
}

// OpenFills returns, oldest first, the fills that belong to the seeded open
// positions.
func (s *FillSegmenter) OpenFills() []models.Trade {
	var out []models.Trade
	for _, fills := range s.walker.OpenFills() {
		out = append(out, fills...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Time == out[j].Time {
			return out[i].ID < out[j].ID
		}
		return out[i].Time < out[j].Time
	})
	return out
}
