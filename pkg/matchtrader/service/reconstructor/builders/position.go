package builders

import (
	"math"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

func BuildPositions(
	events []models.ClosedPosition,
	openIDs map[string]bool,
	leverage uint64,
) []domain.FXPosition {
	groups := helpers.GroupClosed(events)
	positions := make([]domain.FXPosition, 0, len(groups))
	for id, list := range groups {
		positions = append(positions, buildPosition(id, list, leverage, !openIDs[id]))
	}
	fx.SortPositions(positions)
	return positions
}

func buildPosition(id string, events []models.ClosedPosition, leverage uint64, closed bool) domain.FXPosition {
	first, last := events[0], events[len(events)-1]
	side := fx.PositionSide(first.Side)

	var lots, pnl, commission, swap, net float64
	entries := make([]fx.Fill, 0, len(events))
	exits := make([]fx.Fill, 0, len(events))
	openedAt, closedAt := first.OpenedAt(), first.ClosedAt()
	for _, e := range events {
		vol := e.Volume.Float()
		lots += vol
		pnl += e.Profit.Float()
		commission += e.Commission.Float()
		swap += e.Swap.Float()
		net += e.Net()
		entries = append(entries, fx.Fill{Price: e.OpenPrice.Float(), Quantity: vol})
		exits = append(exits, fx.Fill{Price: e.ClosePrice.Float(), Quantity: vol})
		if t := e.OpenedAt(); !t.IsZero() && (openedAt.IsZero() || t.Before(openedAt)) {
			openedAt = t
		}
		if t := e.ClosedAt(); t.After(closedAt) {
			closedAt = t
		}
	}
	amount := fx.Round8(lots)
	entry := fx.Round8(fx.VWAP(entries))
	tp, sl := fx.PriceOrNil(last.TakeProfit.Float()), fx.PriceOrNil(last.StopLoss.Float())

	posID := id
	if !closed {
		posID = id + ":partial"
	}
	pos := domain.FXPosition{
		ID:          posID,
		Side:        side,
		Pair:        helpers.Pair(first.Alias, first.Symbol),
		Symbol:      first.Symbol,
		Amount:      amount,
		EntryPrice:  entry,
		ExitPrice:   fx.Round8(fx.VWAP(exits)),
		Pnl:         fx.Round8(pnl),
		NetPnl:      fx.Round8(net),
		Commission:  fx.Round8(math.Abs(commission)),
		Swap:        fx.Round8(swap),
		TP:          tp,
		SL:          sl,
		Multiplier:  uint32(leverage),
		Closed:      closed,
		CloseReason: last.Reason(),
		Status:      fx.Status(net),
		CreatedAt:   openedAt,
		ClosedAt:    &closedAt,
	}
	fx.ApplyRR(&pos)
	opening := OpeningOrder(posID, first.Symbol, pos.Pair, side, amount, entry, openedAt)
	opening.TP, opening.SL = tp, sl
	pos.Orders = append([]domain.FXOrder{opening}, ClosingOrders(events, posID, side)...)
	return pos
}
