package helpers

import (
	"hyperliquid-trade-reconstructor/internal/connector/hyperliquid/models"
)

type OrderIndex map[int64][]models.HistoricalOrder

func BuildOrderIndex(orders []models.HistoricalOrder) OrderIndex {
	idx := make(OrderIndex)
	for _, o := range orders {
		idx[o.StatusTimestamp] = append(idx[o.StatusTimestamp], o)
	}
	return idx
}
