package builders

import (
	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
)

func buildOrders(
	parts []helpers.FillPart,
	ordersByID map[string]models.Order,
	instrument models.Instrument,
	posID uuid.UUID,
) []domain.Order {
	orders := make([]domain.Order, 0, len(parts))

	for _, part := range parts {
		orderID, err := uuid.NewV7()
		if err != nil {
			continue
		}

		fee, pnl := helpers.AllocateFillStats(part, instrument)
		side := helpers.OrderSideFromOKX(part.Fill.Side)
		amount := helpers.Round8(part.Size)
		price := helpers.Round8(part.Price)

		orderType := "MARKET"
		stopPrice := 0.0
		originalPrice := price
		if order, ok := ordersByID[part.Fill.OrdId]; ok {
			orderType = helpers.OrderTypeFromOKX(order.OrdType)
			stopPrice = helpers.Round8(helpers.MustFloat(order.SlTriggerPx))
			if px := helpers.MustFloat(order.Px); px > 0 {
				originalPrice = helpers.Round8(px)
			}
		}

		orders = append(orders, domain.Order{
			ID:              orderID,
			PositionID:      posID,
			ExchangeOrderID: part.Fill.OrdId,
			Type:            orderType,
			Status:          "FILLED",
			Side:            side,
			Amount:          amount,
			AmountFilled:    amount,
			AveragePrice:    price,
			StopPrice:       stopPrice,
			OriginalPrice:   originalPrice,
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
