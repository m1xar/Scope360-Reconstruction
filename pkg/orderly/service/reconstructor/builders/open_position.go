package builders

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly/models"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/service/reconstructor/helpers"
)

func BuildOpenPositions(positions []models.OrderlyPosition) []domain.OpenPosition {
	out := make([]domain.OpenPosition, 0, len(positions))

	for _, p := range positions {
		if p.PositionQty == 0 {
			continue
		}

		side := "LONG"
		if p.PositionQty < 0 {
			side = "SHORT"
		}
		positionID, err := uuid.NewV7()
		if err != nil {
			continue
		}

		out = append(out, domain.OpenPosition{
			ID:           positionID,
			Pair:         helpers.NormalizeSymbol(p.Symbol),
			Amount:       helpers.Round8(math.Abs(p.PositionQty)),
			Side:         side,
			EntryPrice:   helpers.Round8(p.AverageOpenPrice),
			CurrentPrice: helpers.Round8(p.MarkPrice),
			OpenTime:     time.UnixMilli(p.Timestamp).UTC(),
		})
	}

	return out
}

func BuildOpenOrdersFromTrades(
	trades []models.OrderlyTrade,
	orderMap map[int64]models.OrderlyOrder,
	positionID uuid.UUID,
) []domain.Order {
	out := make([]domain.Order, 0, len(trades))

	for _, t := range trades {
		orderID, err := uuid.NewV7()
		if err != nil {
			continue
		}

		orderType := "MARKET"
		if ord, ok := orderMap[t.OrderID]; ok {
			ot := strings.ToUpper(ord.Type)
			switch {
			case strings.Contains(ot, "LIMIT"):
				orderType = "LIMIT"
			case strings.Contains(ot, "MARKET"):
				orderType = "MARKET"
			}
		}

		doneAt := time.UnixMilli(t.ExecutedTimestamp).UTC()
		price := helpers.Round8(t.ExecutedPrice)
		amount := helpers.Round8(t.ExecutedQuantity)
		fee := helpers.Round8(math.Abs(t.Fee))
		profit := helpers.Round8(t.RealizedPnl)

		out = append(out, domain.Order{
			ID:              orderID,
			PositionID:      positionID,
			ExchangeOrderID: strconv.FormatInt(t.OrderID, 10),
			Type:            orderType,
			Status:          "FILLED",
			Side:            t.Side,
			Amount:          amount,
			AmountFilled:    amount,
			AveragePrice:    price,
			StopPrice:       price,
			OriginalPrice:   price,
			UpdatedAt:       doneAt,
			Trade: domain.Trade{
				OrderID:    orderID,
				Side:       t.Side,
				Price:      price,
				Amount:     amount,
				Commission: fee,
				Profit:     profit,
				DoneAt:     doneAt,
			},
		})
	}

	return out
}
