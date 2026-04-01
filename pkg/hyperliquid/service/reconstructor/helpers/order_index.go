package helpers

import (
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
)

type OrderIndex map[int64][]models.HistoricalOrder

func BuildOrderIndex(orders []models.HistoricalOrder) OrderIndex {
	idx := make(OrderIndex)
	for _, o := range orders {
		idx[o.StatusTimestamp] = append(idx[o.StatusTimestamp], o)
	}
	return idx
}
