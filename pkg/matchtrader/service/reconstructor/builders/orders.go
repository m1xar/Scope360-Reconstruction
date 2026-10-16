package builders

import (
	"math"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

func OpeningOrder(id, symbol, pair, side string, amount, price float64, at time.Time) domain.FXOrder {
	orderSide := fx.OrderSide(side)
	return domain.FXOrder{
		ID: id, PositionID: id, Pair: pair, Symbol: symbol, Type: fx.OrderMarket, Status: fx.StatusFilled, Side: orderSide,
		Amount: amount, AmountFilled: amount, AveragePrice: price, UpdatedAt: at,
		Trade: domain.FXTrade{OrderID: id, Side: orderSide, Price: price, Amount: amount, DoneAt: at},
	}
}

func ClosingOrders(events []models.ClosedPosition, positionID, side string) []domain.FXOrder {
	out := make([]domain.FXOrder, 0, len(events))
	closeSide := fx.ClosingSide(side)
	for _, e := range events {
		id := e.ClosingOrderID
		if id == "" {
			id = e.UID
		}
		amount := e.Volume.Float()
		price := e.ClosePrice.Float()
		at := e.ClosedAt()
		typ := closeOrderType(e.CloseReason)
		order := domain.FXOrder{
			ID: id, PositionID: positionID, Pair: helpers.Pair(e.Alias, e.Symbol), Symbol: e.Symbol, Type: typ, Status: fx.StatusFilled, Side: closeSide,
			Amount: amount, AmountFilled: amount, AveragePrice: price, UpdatedAt: at,
			Trade: domain.FXTrade{
				OrderID: id, Side: closeSide, Price: price, Amount: amount,
				Commission: fx.Round8(math.Abs(e.Commission.Float())), Profit: fx.Round8(e.Profit.Float()), DoneAt: at,
			},
		}
		switch typ {
		case fx.OrderStop:
			order.SL = fx.PriceOrNil(e.StopLoss.Float())
			if order.SL != nil {
				order.StopPrice = *order.SL
			}
		case fx.OrderLimit:
			order.TP = fx.PriceOrNil(e.TakeProfit.Float())
			if order.TP != nil {
				order.OriginalPrice = *order.TP
			}
		}
		out = append(out, order)
	}
	return out
}

func closeOrderType(reason string) string {
	r := strings.ToUpper(strings.TrimSpace(reason))
	if strings.Contains(r, "STOP_LOSS") || strings.Contains(r, "STOPLOSS") {
		return fx.OrderStop
	}
	if strings.Contains(r, "TAKE_PROFIT") || strings.Contains(r, "TAKEPROFIT") {
		return fx.OrderLimit
	}
	for _, tok := range strings.FieldsFunc(r, func(c rune) bool { return c == '_' || c == '-' || c == ' ' }) {
		switch tok {
		case "SL":
			return fx.OrderStop
		case "TP":
			return fx.OrderLimit
		}
	}
	return fx.OrderMarket
}
