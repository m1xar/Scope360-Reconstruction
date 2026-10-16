package builders

import (
	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

func BuildPendingOrders(res *pb.ProtoOAReconcileRes, symbols helpers.Symbols, session *connector.Session) []domain.FXOrder {
	if res == nil {
		return nil
	}
	var out []domain.FXOrder
	for _, order := range res.GetOrder() {
		if order.GetClosingOrder() || order.GetOrderType() == pb.ProtoOAOrderType_STOP_LOSS_TAKE_PROFIT {
			continue
		}
		price := order.GetLimitPrice()
		if price == 0 {
			price = order.GetStopPrice()
		}
		side := TradeSide(order.GetTradeData().GetTradeSide())
		tp, sl := extractProtection([]*pb.ProtoOAOrder{order}, price, side)
		pending := BuildOrder(order, nil, "", tp, sl, symbols, session)
		pending.Status = fx.StatusWorking
		out = append(out, pending)
	}
	return out
}
