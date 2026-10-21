package helpers

import (
	"math"
	"sort"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
)

const sizeEpsilon = 1e-9

type FillPart struct {
	Order models.Order
	Size  float64
	Price float64
	At    time.Time
	Sign  float64
}

type Episode struct {
	Symbol       string
	PositionType int
	OpenSign     float64
	Parts        []FillPart
	OpenAt       time.Time
	CloseAt      time.Time
	PeakSize     float64
	Closed       bool
}

func (e Episode) Side() string {
	if e.OpenSign < 0 {
		return "SHORT"
	}
	return "LONG"
}

func (e Episode) Key() string {
	return PositionKey(e.Symbol, e.PositionType)
}

func OpenEpisodes(fills map[string][]models.Order, contractSizes map[string]float64) map[string]Episode {
	out := make(map[string]Episode, len(fills))
	for key, group := range fills {
		for _, ep := range accumulateEpisodes(group, contractSizes) {
			if !ep.Closed {
				out[key] = ep
			}
		}
	}
	return out
}

func BuildEpisodesFromGroups(groups [][]models.Order, contractSizes map[string]float64) []Episode {
	episodes := make([]Episode, 0, len(groups))
	for _, group := range groups {
		episodes = append(episodes, accumulateEpisodes(group, contractSizes)...)
	}

	sort.SliceStable(episodes, func(i, j int) bool {
		return episodes[i].CloseAt.Before(episodes[j].CloseAt)
	})

	return episodes
}

func SortOrdersOldestFirst(orders []models.Order) {
	sort.SliceStable(orders, func(i, j int) bool {
		if orders[i].UpdateTime == orders[j].UpdateTime {
			return orders[i].OrderId < orders[j].OrderId
		}
		return orders[i].UpdateTime < orders[j].UpdateTime
	})
}

func accumulateEpisodes(group []models.Order, contractSizes map[string]float64) []Episode {
	sorted := make([]models.Order, len(group))
	copy(sorted, group)
	SortOrdersOldestFirst(sorted)

	episodes := make([]Episode, 0, 1)
	var current *Episode
	net := 0.0

	for _, order := range sorted {
		remaining := ContractVolumeToBase(order.DealVol, contractSizes[order.Symbol])
		if remaining <= sizeEpsilon {
			continue
		}
		at := TimeFromMs(order.UpdateTime)
		sign := OrderSign(order.Side)
		price := order.DealAvgPrice

		for remaining > sizeEpsilon {
			if current == nil {
				current = &Episode{
					Symbol:       order.Symbol,
					PositionType: PositionTypeOf(order),
					OpenSign:     sign,
					OpenAt:       at,
				}
				net = 0
			}

			partSize := remaining
			if math.Abs(net) > sizeEpsilon && sign != current.OpenSign {
				partSize = math.Min(math.Abs(net), remaining)
			}

			current.Parts = append(current.Parts, FillPart{
				Order: order,
				Size:  partSize,
				Price: price,
				At:    at,
				Sign:  sign,
			})

			net += sign * partSize
			if absNet := math.Abs(net); absNet > current.PeakSize {
				current.PeakSize = absNet
			}
			remaining = Round8(remaining - partSize)

			if math.Abs(net) < sizeEpsilon {
				current.CloseAt = at
				current.Closed = true
				episodes = append(episodes, *current)
				current = nil
				net = 0
			}
		}
	}

	if current != nil {
		current.CloseAt = current.Parts[len(current.Parts)-1].At
		episodes = append(episodes, *current)
	}

	return episodes
}
