package helpers

import (
	"hyperliquid-trade-reconstructor/internal/hyperliquid/models"
)

type OrderIndex map[int64]models.HistoricalOrder

func BuildOrderIndex(orders []models.HistoricalOrder) OrderIndex {
	idx := make(OrderIndex)
	for _, o := range orders {
		idx[o.StatusTimestamp] = o
	}
	return idx
}
