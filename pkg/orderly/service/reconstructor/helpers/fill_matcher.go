package helpers

import (
	"math"
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly/models"
)

type FillMatch struct {
	Fills []models.OrderlyTrade
	Side  string
}

func MatchFillGroups(
	trades []models.OrderlyTrade,
	orderMap map[int64]models.OrderlyOrder,
) ([]FillMatch, map[int]struct{}) {
	const sizeEpsilon = 1e-9

	sort.Slice(trades, func(i, j int) bool {
		return trades[i].ExecutedTimestamp < trades[j].ExecutedTimestamp
	})

	usedFills := make(map[int]struct{})
	matched := make(map[int]struct{})
	matches := make([]FillMatch, 0)

	for i := 0; i < len(trades); i++ {
		t := trades[i]
		if _, ok := usedFills[t.ID]; ok {
			continue
		}

		if isCloseTrade(t, orderMap) {
			continue
		}

		symbol := t.Symbol
		side := t.Side
		positionSide := TradeSideToPositionSide(side)
		size := t.ExecutedQuantity

		recon := []models.OrderlyTrade{t}
		usedFills[t.ID] = struct{}{}

		for j := i + 1; j < len(trades); j++ {
			n := trades[j]
			if _, ok := usedFills[n.ID]; ok {
				continue
			}
			if n.Symbol != symbol {
				continue
			}

			if n.Side == side && !isCloseTrade(n, orderMap) {

				size += n.ExecutedQuantity
				recon = append(recon, n)
				usedFills[n.ID] = struct{}{}
			} else if n.Side != side || isCloseTrade(n, orderMap) {

				size -= n.ExecutedQuantity
				recon = append(recon, n)
				usedFills[n.ID] = struct{}{}

				if math.Abs(size) < sizeEpsilon {
					cp := make([]models.OrderlyTrade, len(recon))
					copy(cp, recon)
					matches = append(matches, FillMatch{Fills: cp, Side: positionSide})
					for _, fl := range cp {
						matched[fl.ID] = struct{}{}
					}
					break
				}
			}
		}
	}

	return matches, matched
}

func isCloseTrade(t models.OrderlyTrade, orderMap map[int64]models.OrderlyOrder) bool {
	if t.RealizedPnl != 0 {
		return true
	}

	if ord, ok := orderMap[t.OrderID]; ok {
		return ord.ReduceOnly
	}

	return false
}
