package builders

import (
	"math"
	"sort"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

func BuildOrders(orders []models.Order, positionID string, lotSizes map[string]float64) []domain.FXOrder {
	tp, sl := Protection(orders, time.Time{})
	out := make([]domain.FXOrder, 0, len(orders))
	for _, o := range orders {
		order := BuildOrder(o, positionID, lotSizes)
		switch {
		case !o.Closing():
			order.TP, order.SL = tp, sl
		case o.Status == models.StatusRejected:
		case o.Type == fx.OrderLimit:
			order.TP = fx.PriceOrNil(order.OriginalPrice)
		case o.Type == fx.OrderStop:
			order.SL = fx.PriceOrNil(order.OriginalPrice)
		}
		out = append(out, order)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.Before(out[j].UpdatedAt) })
	return out
}

func BuildOrder(o models.Order, positionID string, lotSizes map[string]float64) domain.FXOrder {
	leg := o.Leg()
	fills := helpers.Fills(o)
	lots := func(units float64) float64 { return helpers.Lots(lotSizes, o.Symbol(), units) }
	filled := lots(math.Max(helpers.Quantity(fills), math.Abs(leg.FilledQuantity.Float())))
	amount := lots(math.Abs(leg.Quantity.Float()))
	if amount == 0 {
		amount = filled
	}
	avg := leg.AveragePrice.Float()
	if avg == 0 {
		avg = helpers.VWAP(fills)
	}
	price := leg.Price.Float()
	out := domain.FXOrder{
		ID:            o.OrderID.String(),
		PositionID:    positionID,
		Pair:          fx.NormalizePair(o.Symbol()),
		Symbol:        o.Symbol(),
		Type:          o.Type,
		Status:        helpers.OrderStatus(o),
		Side:          o.OrderSide(),
		Amount:        amount,
		AmountFilled:  filled,
		AveragePrice:  avg,
		OriginalPrice: price,
		UpdatedAt:     o.TransactionTime.Time,
	}
	if o.Type == fx.OrderStop {
		out.StopPrice = price
	}
	if out.UpdatedAt.IsZero() {
		out.UpdatedAt = o.IssueTime.Time
	}
	if len(fills) > 0 {
		cash := helpers.SumTransactions(o.CashTransactions)
		out.Trade = domain.FXTrade{
			OrderID:    out.ID,
			Side:       out.Side,
			Price:      helpers.VWAP(fills),
			Amount:     lots(helpers.Quantity(fills)),
			Commission: fx.Round8(math.Abs(cash.Commission)),
			Profit:     fx.Round8(cash.Settlement),
			DoneAt:     fills[len(fills)-1].Time,
		}
	}
	return out
}
