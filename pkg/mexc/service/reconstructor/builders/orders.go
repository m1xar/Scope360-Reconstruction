package builders

import (
	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
)

func buildOrders(parts []helpers.FillPart, contractSizes map[string]float64, posID uuid.UUID) []domain.Order {
	orders := make([]domain.Order, 0, len(parts))

	for _, part := range parts {
		orderID, err := uuid.NewV7()
		if err != nil {
			continue
		}

		fee, pnl := helpers.AllocateFillStats(part, contractSizes)
		side := helpers.OrderSideFromMEXC(part.Order.Side)
		amount := helpers.Round8(part.Size)
		price := helpers.Round8(part.Price)

		orders = append(orders, domain.Order{
			ID:              orderID,
			PositionID:      posID,
			ExchangeOrderID: part.Order.OrderId,
			Type:            helpers.OrderTypeFromMEXC(part.Order.OrderType),
			Status:          "FILLED",
			Side:            side,
			Amount:          amount,
			AmountFilled:    amount,
			AveragePrice:    price,
			StopPrice:       helpers.Round8(part.Order.StopLossPrice),
			OriginalPrice:   helpers.Round8(part.Order.Price),
			UpdatedAt:       part.At,
			Trade: domain.Trade{
				OrderID:    orderID,
				Side:       side,
				Price:      price,
				Amount:     amount,
				Commission: helpers.Round8(fee),
				Profit:     helpers.Round8(pnl),
				DoneAt:     part.At,
			},
		})
	}

	return orders
}
