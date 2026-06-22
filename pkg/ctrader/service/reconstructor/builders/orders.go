package builders

import (
	"strconv"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func BuildOrders(orders []*pb.ProtoOAOrder, deals []*pb.ProtoOADeal, positionID string, session *connector.Session) []domain.FXOrder {
	dealByOrderID := make(map[int64]*pb.ProtoOADeal, len(deals))
	for _, deal := range deals {
		dealByOrderID[deal.GetOrderId()] = deal
	}
	out := make([]domain.FXOrder, 0, len(orders))
	for _, order := range orders {
		tradeData := order.GetTradeData()
		orderID := order.GetOrderId()
		deal := dealByOrderID[orderID]
		id := strconv.FormatInt(orderID, 10)
		updatedAt := helpers.TimeFromMillis(order.GetUtcLastUpdateTimestamp())
		if updatedAt.IsZero() && tradeData != nil {
			updatedAt = helpers.TimeFromMillis(tradeData.GetOpenTimestamp())
		}
		avgPrice := order.GetExecutionPrice()
		if avgPrice == 0 {
			avgPrice = order.GetLimitPrice()
		}
		domainOrder := domain.FXOrder{
			ID:            id,
			PositionID:    positionID,
			Type:          orderType(order),
			Status:        filledOrderStatus(order, deal),
			Side:          ExecutionSide(orderSide(order, deal)),
			Amount:        volumeToAmount(order.GetTradeData().GetVolume()),
			AmountFilled:  volumeToAmount(order.GetExecutedVolume()),
			AveragePrice:  avgPrice,
			StopPrice:     order.GetStopPrice(),
			OriginalPrice: order.GetLimitPrice(),
			UpdatedAt:     updatedAt,
		}
		if deal != nil {
			domainOrder.Trade = domain.FXTrade{
				OrderID:    id,
				Side:       ExecutionSide(deal.GetTradeSide()),
				Price:      deal.GetExecutionPrice(),
				Amount:     volumeToAmount(deal.GetFilledVolume()),
				Commission: dealCommission(deal, session),
				Profit:     dealProfit(deal, session),
				DoneAt:     helpers.TimeFromMillis(deal.GetExecutionTimestamp()),
			}
		}
		out = append(out, domainOrder)
	}
	return out
}

func BuildOrdersFromDeals(deals []*pb.ProtoOADeal, positionID string, session *connector.Session) []domain.FXOrder {
	out := make([]domain.FXOrder, 0, len(deals))
	for _, deal := range deals {
		orderID := deal.GetOrderId()
		id := strconv.FormatInt(orderID, 10)
		doneAt := helpers.TimeFromMillis(deal.GetExecutionTimestamp())
		out = append(out, domain.FXOrder{
			ID:           id,
			PositionID:   positionID,
			Type:         "MARKET",
			Status:       "FILLED",
			Side:         ExecutionSide(deal.GetTradeSide()),
			Amount:       volumeToAmount(deal.GetVolume()),
			AmountFilled: volumeToAmount(deal.GetFilledVolume()),
			AveragePrice: deal.GetExecutionPrice(),
			UpdatedAt:    doneAt,
			Trade: domain.FXTrade{
				OrderID:    id,
				Side:       ExecutionSide(deal.GetTradeSide()),
				Price:      deal.GetExecutionPrice(),
				Amount:     volumeToAmount(deal.GetFilledVolume()),
				Commission: dealCommission(deal, session),
				Profit:     dealProfit(deal, session),
				DoneAt:     doneAt,
			},
		})
	}
	return out
}

func TradeSide(side pb.ProtoOATradeSide) string {
	if side == pb.ProtoOATradeSide_BUY {
		return "LONG"
	}
	return "SHORT"
}

func ExecutionSide(side pb.ProtoOATradeSide) string {
	if side == pb.ProtoOATradeSide_BUY {
		return "BUY"
	}
	return "SELL"
}

func orderSide(order *pb.ProtoOAOrder, deal *pb.ProtoOADeal) pb.ProtoOATradeSide {
	if order != nil && order.GetTradeData() != nil {
		return order.GetTradeData().GetTradeSide()
	}
	if deal != nil {
		return deal.GetTradeSide()
	}
	return pb.ProtoOATradeSide_BUY
}

func volumeToAmount(volume int64) float64 {
	return float64(volume) / 100
}

func orderType(order *pb.ProtoOAOrder) string {
	if order == nil {
		return "MARKET"
	}
	switch order.GetOrderType() {
	case pb.ProtoOAOrderType_MARKET, pb.ProtoOAOrderType_MARKET_RANGE:
		return "MARKET"
	case pb.ProtoOAOrderType_LIMIT:
		return "LIMIT"
	case pb.ProtoOAOrderType_STOP:
		return "STOP"
	case pb.ProtoOAOrderType_STOP_LIMIT:
		return "STOP_LIMIT"
	case pb.ProtoOAOrderType_STOP_LOSS_TAKE_PROFIT:
		return "STOP"
	default:
		return order.GetOrderType().String()
	}
}

func orderStatus(status pb.ProtoOAOrderStatus) string {
	switch status {
	case pb.ProtoOAOrderStatus_ORDER_STATUS_FILLED:
		return "FILLED"
	case pb.ProtoOAOrderStatus_ORDER_STATUS_ACCEPTED:
		return "ACCEPTED"
	case pb.ProtoOAOrderStatus_ORDER_STATUS_REJECTED:
		return "REJECTED"
	case pb.ProtoOAOrderStatus_ORDER_STATUS_EXPIRED:
		return "EXPIRED"
	case pb.ProtoOAOrderStatus_ORDER_STATUS_CANCELLED:
		return "CANCELLED"
	default:
		return status.String()
	}
}

func filledOrderStatus(order *pb.ProtoOAOrder, deal *pb.ProtoOADeal) string {
	if deal != nil || (order != nil && order.GetExecutedVolume() > 0) {
		return "FILLED"
	}
	if order == nil {
		return "FILLED"
	}
	return orderStatus(order.GetOrderStatus())
}

func dealCommission(deal *pb.ProtoOADeal, session *connector.Session) float64 {
	if deal == nil {
		return 0
	}
	digits := helpers.MoneyDigits(session, deal)
	value := deal.GetCommission()
	if detail := deal.GetClosePositionDetail(); detail != nil && detail.GetCommission() != 0 {
		value = detail.GetCommission()
	}
	return helpers.Round8(helpers.Abs(helpers.Money(value, digits)))
}

func dealProfit(deal *pb.ProtoOADeal, session *connector.Session) float64 {
	if deal == nil {
		return 0
	}
	detail := deal.GetClosePositionDetail()
	if detail == nil {
		return 0
	}
	return helpers.Round8(helpers.Money(detail.GetGrossProfit(), helpers.MoneyDigits(session, deal)))
}
