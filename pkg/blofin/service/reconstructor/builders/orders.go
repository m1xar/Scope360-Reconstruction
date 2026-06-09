package builders

import (
	"math"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func BuildOrders(orders []models.Order, posID uuid.UUID, contractValue float64) []domain.Order {
	result := make([]domain.Order, 0, len(orders))
	for _, ord := range orders {
		if !helpers.IsFilled(ord) {
			continue
		}

		orderID, err := uuid.NewV7()
		if err != nil {
			continue
		}

		side := helpers.OrderSideFromRaw(ord.Side)
		avgPx := helpers.Round8(helpers.MustFloat(ord.AveragePrice))
		fillSz := helpers.Round8(helpers.MustFloat(ord.FilledSize) * contractValue)
		size := helpers.Round8(helpers.MustFloat(ord.Size) * contractValue)
		fee := helpers.Round8(math.Abs(helpers.MustFloat(ord.Fee)))
		pnl := helpers.Round8(helpers.MustFloat(ord.Pnl))
		doneAt := helpers.TimeFromMs(timestampOrFallback(ord.UpdateTime, ord.CreateTime))

		result = append(result, domain.Order{
			ID:              orderID,
			PositionID:      posID,
			ExchangeOrderID: helpers.OrderID(ord),
			Type:            helpers.OrderTypeFromRaw(ord),
			Status:          "FILLED",
			Side:            side,
			Amount:          size,
			AmountFilled:    fillSz,
			AveragePrice:    avgPx,
			StopPrice:       helpers.StopPrice(ord),
			OriginalPrice:   helpers.Round8(helpers.MustFloat(ord.Price)),
			UpdatedAt:       doneAt,
			Trade: domain.Trade{
				OrderID:    orderID,
				Side:       side,
				Price:      avgPx,
				Amount:     fillSz,
				Commission: fee,
				Profit:     pnl,
				DoneAt:     doneAt,
			},
		})
	}
	return result
}

func timestampOrFallback(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}
