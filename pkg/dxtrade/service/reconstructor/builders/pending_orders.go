package builders

import (
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

func BuildPendingOrders(working []models.Order, positions []models.Position, lotSizes map[string]float64) []domain.FXOrder {
	open := make(map[string]bool, len(positions))
	for _, p := range positions {
		open[p.PositionCode.String()] = true
	}
	var out []domain.FXOrder
	for _, o := range working {
		if helpers.Final(o) || o.Closing() || open[o.PositionCode()] {
			continue
		}
		order := BuildOrder(o, "", lotSizes)
		order.TP, order.SL = workingProtection(working, o.PositionCode())
		out = append(out, order)
	}
	return out
}

func workingProtection(working []models.Order, positionCode string) (tp, sl *float64) {
	for _, o := range working {
		if !o.Closing() || helpers.Final(o) || o.PositionCode() != positionCode {
			continue
		}
		price := fx.PriceOrNil(o.Leg().Price.Float())
		switch o.Type {
		case fx.OrderLimit:
			tp = price
		case fx.OrderStop:
			sl = price
		}
	}
	return tp, sl
}
