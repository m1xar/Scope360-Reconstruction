package builders

import (
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

func BuildFXPositions(
	orders []models.Order,
	ledger []helpers.CashEntry,
	open []models.Position,
	instruments map[string]models.InstrumentDetails,
	lotSizes map[string]float64,
	hedging bool,
) []domain.FXPosition {
	var out []domain.FXPosition
	if hedging {
		out = buildHedging(orders, ledger, open, instruments, lotSizes)
	} else {
		out = buildNetting(orders, open, instruments, lotSizes)
	}
	fx.SortPositions(out)
	return out
}

func buildHedging(
	orders []models.Order,
	ledger []helpers.CashEntry,
	open []models.Position,
	instruments map[string]models.InstrumentDetails,
	lotSizes map[string]float64,
) []domain.FXPosition {
	openCodes := make(map[string]bool, len(open))
	for _, p := range open {
		openCodes[p.PositionCode.String()] = true
	}
	groups := make(map[string][]models.Order)
	var codes []string
	for _, o := range orders {
		code := o.PositionCode()
		if code == "" {
			continue
		}
		if _, ok := groups[code]; !ok {
			codes = append(codes, code)
		}
		groups[code] = append(groups[code], o)
	}
	cash := helpers.ByPosition(ledger)

	var out []domain.FXPosition
	for _, code := range codes {
		group := groups[code]
		var opens, closes []helpers.Fill
		side := ""
		for _, o := range group {
			fills := helpers.Fills(o)
			if o.Closing() {
				closes = append(closes, fills...)
				continue
			}
			if side == "" && len(fills) > 0 {
				side = fx.PositionSide(o.OrderSide())
			}
			opens = append(opens, fills...)
		}
		openQty, closeQty := helpers.Quantity(opens), helpers.Quantity(closes)
		if openQty <= helpers.Epsilon || closeQty <= helpers.Epsilon {
			continue
		}
		helpers.SortFills(opens)
		helpers.SortFills(closes)
		closed := !openCodes[code] && closeQty+helpers.Epsilon*max(1, openQty) >= openQty
		tp, sl := Protection(group, closes[len(closes)-1].Time)
		posID := code
		if !closed {
			posID = code + ":partial"
		}
		out = append(out, buildPosition(posID, side, opens, closes, helpers.SumEntries(cash[code]), BuildOrders(group, posID, lotSizes), tp, sl, instruments, lotSizes, closed, closeReason(group, closes[len(closes)-1])))
	}
	return out
}

func Protection(orders []models.Order, until time.Time) (tp, sl *float64) {
	var tpAt, slAt time.Time
	for _, o := range orders {
		if !o.Closing() || o.Status == models.StatusRejected || (!until.IsZero() && o.IssueTime.After(until)) {
			continue
		}
		price := o.Leg().Price.Float()
		if price <= 0 {
			price = o.Leg().AveragePrice.Float()
		}
		p := fx.PriceOrNil(price)
		if p == nil {
			continue
		}
		switch o.Type {
		case fx.OrderLimit:
			if tp == nil || !o.IssueTime.Before(tpAt) {
				tp, tpAt = p, o.IssueTime.Time
			}
		case fx.OrderStop:
			if sl == nil || !o.IssueTime.Before(slAt) {
				sl, slAt = p, o.IssueTime.Time
			}
		}
	}
	return tp, sl
}

func buildPosition(
	id, side string,
	opens, closes []helpers.Fill,
	cash helpers.CashSums,
	orders []domain.FXOrder,
	tp, sl *float64,
	instruments map[string]models.InstrumentDetails,
	lotSizes map[string]float64,
	closed bool,
	reason string,
) domain.FXPosition {
	closedAt := closes[len(closes)-1].Time
	net := cash.Net()
	symbol := opens[0].Symbol
	amount := helpers.Quantity(opens)
	if !closed {
		amount = helpers.Quantity(closes)
	}
	pos := domain.FXPosition{
		ID:          id,
		Side:        side,
		Pair:        fx.NormalizePair(symbol),
		Symbol:      symbol,
		Amount:      helpers.Lots(lotSizes, symbol, amount),
		EntryPrice:  helpers.VWAP(opens),
		ExitPrice:   helpers.VWAP(closes),
		Pnl:         fx.Round8(cash.Settlement),
		NetPnl:      fx.Round8(net),
		Commission:  fx.Round8(abs(cash.Commission)),
		Swap:        fx.Round8(cash.Financing),
		TP:          tp,
		SL:          sl,
		Multiplier:  helpers.Multiplier(instruments[symbol]),
		Closed:      closed,
		CloseReason: reason,
		Status:      fx.Status(net),
		CreatedAt:   opens[0].Time,
		ClosedAt:    &closedAt,
		Orders:      orders,
	}
	fx.ApplyRR(&pos)
	return pos
}

func closeReason(orders []models.Order, last helpers.Fill) string {
	for _, o := range orders {
		if o.OrderID.String() != last.OrderID {
			continue
		}
		switch o.Type {
		case fx.OrderLimit:
			return "TP"
		case fx.OrderStop:
			return "SL"
		}
	}
	return "USER"
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
