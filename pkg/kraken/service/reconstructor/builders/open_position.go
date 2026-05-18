package builders

import (
	"strings"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/helpers"
)

func BuildOpenPosition(pos models.OpenPosition, ticker models.Ticker) domain.OpenPosition {
	openTime, _ := helpers.ParseTime(pos.FillTime)
	side := "LONG"
	if strings.EqualFold(pos.Side, "short") {
		side = "SHORT"
	}

	pair := ticker.Pair
	if pair == "" {
		pair = helpers.NormalizePairFallback(pos.Symbol)
	}
	positionID, err := uuid.NewV7()
	if err != nil {
		positionID = uuid.Nil
	}

	return domain.OpenPosition{
		ID:           positionID,
		Pair:         strings.ToUpper(strings.ReplaceAll(pair, "_", "")),
		Amount:       helpers.Round8(pos.Size.Float64()),
		Side:         side,
		EntryPrice:   helpers.Round8(pos.Price.Float64()),
		CurrentPrice: helpers.Round8(ticker.MarkPrice.Float64()),
		OpenTime:     openTime,
	}
}

func BuildOpenOrdersFromFills(fills []models.Fill, positionID uuid.UUID) []domain.Order {
	out := make([]domain.Order, 0, len(fills))

	for _, fill := range fills {
		doneAt, err := helpers.ParseTime(fill.FillTime)
		if err != nil {
			continue
		}

		orderID, err := uuid.NewV7()
		if err != nil {
			continue
		}

		side := helpers.OrderSide(fill.Side)
		price := helpers.Round8(fill.Price.Float64())
		amount := helpers.Round8(fill.Size.Float64())

		out = append(out, domain.Order{
			ID:              orderID,
			PositionID:      positionID,
			ExchangeOrderID: fill.OrderID,
			Type:            helpers.OrderType(fill.FillType),
			Status:          "FILLED",
			Side:            side,
			Amount:          amount,
			AmountFilled:    amount,
			AveragePrice:    price,
			OriginalPrice:   price,
			UpdatedAt:       doneAt,
			Trade: domain.Trade{
				OrderID: orderID,
				Side:    side,
				Price:   price,
				Amount:  amount,
				DoneAt:  doneAt,
			},
		})
	}

	return out
}
