package helpers

import (
	"math"
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/reverse"
)

func FillKey(fill models.Fill) string {
	return PositionKey(fill.InstId, fill.PosSide)
}

func FillDelta(fill models.Fill) float64 {
	return fillSign(fill) * MustFloat(fill.FillSz)
}

func SeedFromOpenPositions(positions []models.OpenPosition) map[string]float64 {
	out := make(map[string]float64, len(positions))
	for _, pos := range positions {
		size := MustFloat(pos.Pos)
		if size == 0 {
			continue
		}
		if SideFromPosSide(pos.PosSide, pos.Pos) == "SHORT" {
			size = -math.Abs(size)
		} else {
			size = math.Abs(size)
		}
		out[PositionKey(pos.InstId, pos.PosSide)] += size
	}
	return out
}

type FillSegmenter struct {
	walker *reverse.Walker[models.Fill]
}

func NewFillSegmenter(openPositions []models.OpenPosition) *FillSegmenter {
	return &FillSegmenter{walker: reverse.NewSeededWalker[models.Fill](SeedFromOpenPositions(openPositions))}
}

func (s *FillSegmenter) PushOlderBatch(fills []models.Fill) [][]models.Fill {
	batch := make([]models.Fill, len(fills))
	copy(batch, fills)
	sort.SliceStable(batch, func(i, j int) bool {
		ti, tj := MustInt64(batch[i].Ts), MustInt64(batch[j].Ts)
		if ti == tj {
			return MustInt64(batch[i].BillId) > MustInt64(batch[j].BillId)
		}
		return ti > tj
	})

	groups := make([][]models.Fill, 0)
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

func (s *FillSegmenter) Resolved() bool {
	return s.walker.Resolved()
}

func (s *FillSegmenter) OpenFills() map[string][]models.Fill {
	return s.walker.OpenFills()
}
