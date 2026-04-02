package helpers

import (
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
)

func GroupOrdersBySymbol(orders []models.Order) map[string][]models.Order {
	idx := make(map[string][]models.Order)
	for _, o := range orders {
		idx[o.Symbol] = append(idx[o.Symbol], o)
	}
	return idx
}

func MatchOrdersToPosition(hp models.HistoryPosition, ordersBySymbol map[string][]models.Order) []models.Order {
	symOrders := ordersBySymbol[hp.Symbol]
	if len(symOrders) == 0 {
		return nil
	}

	isLong := hp.PositionType == 1

	matched := make([]models.Order, 0)
	for _, ord := range symOrders {
		if ord.CreateTime < hp.CreateTime || ord.UpdateTime > hp.UpdateTime {
			continue
		}

		orderForLong := IsOrderForLong(ord.Side)
		if isLong != orderForLong {
			continue
		}

		matched = append(matched, ord)
	}
	return matched
}
