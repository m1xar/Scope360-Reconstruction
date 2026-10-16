package builders

import (
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
	"math"
)

func BuildOpenPositions(open []models.OpenPosition, events []models.ClosedPosition) []domain.FXOpenPosition {
	partials := helpers.GroupClosed(events)
	out := make([]domain.FXOpenPosition, 0, len(open))
	for _, p := range open {
		side := fx.PositionSide(p.Side)
		openedAt := p.OpenedAt()
		amount := p.Volume.Float()
		closes := partials[p.ID]
		lots := p.Volume.Float()
		for _, e := range closes {
			lots += e.Volume.Float()
		}
		entry := p.OpenPrice.Float()

		pair := helpers.Pair(p.Alias, p.Symbol)
		sl, tp := fx.PriceOrNil(p.StopLoss.Float()), fx.PriceOrNil(p.TakeProfit.Float())
		opening := OpeningOrder(p.ID, p.Symbol, pair, side, lots, entry, openedAt)
		opening.TP, opening.SL = tp, sl
		orders := append([]domain.FXOrder{opening}, ClosingOrders(closes, p.ID, side)...)
		closeSide := fx.ClosingSide(side)
		if sl != nil {
			orders = append(orders, domain.FXOrder{
				ID: p.ID + "-SL", PositionID: p.ID, Pair: pair, Symbol: p.Symbol, Type: fx.OrderStop, Status: fx.StatusWorking, Side: closeSide,
				Amount: amount, StopPrice: *sl, OriginalPrice: *sl, UpdatedAt: openedAt,
			})
		}
		if tp != nil {
			orders = append(orders, domain.FXOrder{
				ID: p.ID + "-TP", PositionID: p.ID, Pair: pair, Symbol: p.Symbol, Type: fx.OrderLimit, Status: fx.StatusWorking, Side: closeSide,
				Amount: amount, OriginalPrice: *tp, UpdatedAt: openedAt,
			})
		}
		out = append(out, domain.FXOpenPosition{
			ID:            p.ID,
			Pair:          pair,
			Symbol:        p.Symbol,
			Amount:        amount,
			Side:          side,
			EntryPrice:    entry,
			CurrentPrice:  p.CurrentPrice.Float(),
			UnrealizedPnl: fx.Round8(p.Profit.Float()),
			Commission:    fx.Round8(math.Abs(p.Commission.Float())),
			Swap:          fx.Round8(p.Swap.Float()),
			Margin:        fx.Round8(p.Margin.Float()),
			OpenTime:      openedAt,
			Orders:        orders,
		})
	}
	return out
}
