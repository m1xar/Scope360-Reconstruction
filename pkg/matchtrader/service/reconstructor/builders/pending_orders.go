package builders

import (
	"sort"
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

func BuildPendingOrders(orders []models.ActiveOrder) []domain.FXOrder {
	out := make([]domain.FXOrder, 0, len(orders))
	for _, o := range orders {
		typ := fx.OrderLimit
		if strings.Contains(strings.ToUpper(o.Type), "STOP") {
			typ = fx.OrderStop
		}
		price := o.ActivationPrice.Float()
		order := domain.FXOrder{
			ID: o.ID, Pair: helpers.Pair(o.Alias, o.Symbol), Symbol: o.Symbol, Type: typ, Status: fx.StatusWorking,
			Side: strings.ToUpper(strings.TrimSpace(o.Side)), Amount: o.Volume.Float(),
			OriginalPrice: price, TP: fx.PriceOrNil(o.TakeProfit.Float()), SL: fx.PriceOrNil(o.StopLoss.Float()),
			UpdatedAt: o.CreatedAt(),
		}
		if typ == fx.OrderStop {
			order.StopPrice = price
		}
		out = append(out, order)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.Before(out[j].UpdatedAt) })
	return out
}
