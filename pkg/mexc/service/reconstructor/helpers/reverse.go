package helpers

import (
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/reverse"
)

func FillKey(order models.Order) string {
	return PositionKey(order.Symbol, PositionTypeOf(order))
}

func FillDelta(order models.Order, contractSizes map[string]float64) float64 {
	return OrderSign(order.Side) * ContractVolumeToBase(order.DealVol, contractSizes[order.Symbol])
}

func SeedFromOpenPositions(positions []models.OpenPosition, contractSizes map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(positions))
	for _, pos := range positions {
		size := ContractVolumeToBase(pos.HoldVol, contractSizes[pos.Symbol])
		if size <= 0 {
			continue
		}
		if pos.PositionType == 2 {
			size = -size
		}
		out[PositionKey(pos.Symbol, pos.PositionType)] += size
	}
	return out
}

type FillSegmenter struct {
	walker        *reverse.Walker[models.Order]
	contractSizes map[string]float64
	seen          map[string]struct{}
}

func NewFillSegmenter(openPositions []models.OpenPosition, contractSizes map[string]float64) *FillSegmenter {
	return &FillSegmenter{
		walker:        reverse.NewSeededWalker[models.Order](SeedFromOpenPositions(openPositions, contractSizes)),
		contractSizes: contractSizes,
		seen:          make(map[string]struct{}),
	}
}

func (s *FillSegmenter) PushOlderBatch(orders []models.Order) [][]models.Order {
	batch := make([]models.Order, 0, len(orders))
	for _, order := range orders {
		if _, ok := s.seen[order.OrderId]; ok {
			continue
		}
		s.seen[order.OrderId] = struct{}{}
		batch = append(batch, order)
	}
	sort.SliceStable(batch, func(i, j int) bool {
		if batch[i].UpdateTime == batch[j].UpdateTime {
			return batch[i].OrderId > batch[j].OrderId
		}
		return batch[i].UpdateTime > batch[j].UpdateTime
	})

	groups := make([][]models.Order, 0)
	for _, order := range batch {
		delta := FillDelta(order, s.contractSizes)
		if delta == 0 {
			continue
		}
		if group, ok := s.walker.Push(FillKey(order), delta, order); ok {
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

func (s *FillSegmenter) OpenFills() map[string][]models.Order {
	out := s.walker.OpenFills()
	for _, fills := range out {
		SortOrdersOldestFirst(fills)
	}
	return out
}
