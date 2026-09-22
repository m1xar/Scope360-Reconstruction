package builders

import (
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/helpers"
)

// IsOpeningEvent reports whether the update started the current position:
// from flat, or by flipping through zero.
func IsOpeningEvent(upd models.PositionUpdate) bool {
	old, cur := upd.OldPosition.Float64(), upd.NewPosition.Float64()
	return (old == 0 && cur != 0) || old*cur < 0
}

// BuildOpenOrdersFromEvents turns the execution events of an open position
// (oldest first, starting with its opening event) into orders. Unlike fills
// the events include liquidations and ADL, so they add up to the live size.
func BuildOpenOrdersFromEvents(events []models.PositionUpdate, positionID uuid.UUID) []domain.Order {
	out := make([]domain.Order, 0, len(events))
	for i, upd := range events {
		old, cur := upd.OldPosition.Float64(), upd.NewPosition.Float64()
		change := cur - old
		if change == 0 {
			continue
		}
		amount := math.Abs(change)
		if i == 0 && old*cur < 0 {
			// A flip: only the part past zero belongs to this position.
			amount = math.Abs(cur)
		}

		orderID, err := uuid.NewV7()
		if err != nil {
			continue
		}
		side := "BUY"
		if change < 0 {
			side = "SELL"
		}
		price := helpers.Round8(upd.ExecutionPrice.Float64())
		at := EventTime(upd)
		amount = helpers.Round8(amount)

		out = append(out, domain.Order{
			ID:              orderID,
			PositionID:      positionID,
			ExchangeOrderID: upd.ExecutionUID,
			Type:            "MARKET",
			Status:          "FILLED",
			Side:            side,
			Amount:          amount,
			AmountFilled:    amount,
			AveragePrice:    price,
			OriginalPrice:   price,
			UpdatedAt:       at,
			Trade: domain.Trade{
				OrderID:    orderID,
				Side:       side,
				Price:      price,
				Amount:     amount,
				Commission: helpers.Round8(math.Abs(upd.Fee.Float64())),
				Profit:     helpers.Round8(upd.RealizedPnL.Float64()),
				DoneAt:     at,
			},
		})
	}
	return out
}

// EventTime is when the update happened: the fill time, or the event's own
// timestamp for updates that are not executions.
func EventTime(upd models.PositionUpdate) time.Time {
	ms := upd.FillTime
	if ms == 0 {
		ms = upd.Timestamp
	}
	return time.UnixMilli(ms).UTC()
}
