package builders

import (
	"math"
	"sort"
	"strconv"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

func BuildOrders(
	orders []*pb.ProtoOAOrder,
	deals []*pb.ProtoOADeal,
	positionID string,
	tp, sl *float64,
	symbols helpers.Symbols,
	session *connector.Session,
) []domain.FXOrder {
	dealsByOrder := make(map[int64][]*pb.ProtoOADeal, len(deals))
	for _, deal := range deals {
		dealsByOrder[deal.GetOrderId()] = append(dealsByOrder[deal.GetOrderId()], deal)
	}
	out := make([]domain.FXOrder, 0, len(orders))
	seen := make(map[int64]bool, len(orders))
	for _, order := range orders {
		seen[order.GetOrderId()] = true
		out = append(out, BuildOrder(order, dealsByOrder[order.GetOrderId()], positionID, tp, sl, symbols, session))
	}
	for orderID, orderDeals := range dealsByOrder {
		if !seen[orderID] {
			out = append(out, orderFromDeals(orderID, orderDeals, positionID, symbols, session))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.Before(out[j].UpdatedAt) })
	return out
}

func BuildOrder(
	order *pb.ProtoOAOrder,
	deals []*pb.ProtoOADeal,
	positionID string,
	positionTP, positionSL *float64,
	symbols helpers.Symbols,
	session *connector.Session,
) domain.FXOrder {
	tradeData := order.GetTradeData()
	symbolID := tradeData.GetSymbolId()
	id := strconv.FormatInt(order.GetOrderId(), 10)
	updatedAt := helpers.TimeFromMillis(order.GetUtcLastUpdateTimestamp())
	if updatedAt.IsZero() {
		updatedAt = helpers.TimeFromMillis(tradeData.GetOpenTimestamp())
	}
	avgPrice := order.GetExecutionPrice()
	if avgPrice == 0 {
		avgPrice = order.GetLimitPrice()
	}
	out := domain.FXOrder{
		ID:            id,
		PositionID:    positionID,
		Pair:          symbols.Pair(symbolID),
		Symbol:        symbols.Name(symbolID),
		Type:          orderType(order),
		Status:        orderStatus(order, len(deals) > 0),
		Side:          executionSide(tradeData.GetTradeSide()),
		Amount:        symbols.Lots(symbolID, tradeData.GetVolume()),
		AmountFilled:  symbols.Lots(symbolID, order.GetExecutedVolume()),
		AveragePrice:  avgPrice,
		StopPrice:     order.GetStopPrice(),
		OriginalPrice: order.GetLimitPrice(),
		UpdatedAt:     updatedAt,
	}
	switch {
	case order.GetOrderType() == pb.ProtoOAOrderType_STOP_LOSS_TAKE_PROFIT && order.StopLoss != nil:
		out.SL = fx.PriceOrNil(order.GetStopLoss())
		out.StopPrice = order.GetStopLoss()
	case order.GetOrderType() == pb.ProtoOAOrderType_STOP_LOSS_TAKE_PROFIT && order.TakeProfit != nil:
		out.TP = fx.PriceOrNil(order.GetTakeProfit())
		out.OriginalPrice = order.GetTakeProfit()
	case !order.GetClosingOrder():
		out.TP, out.SL = positionTP, positionSL
	}
	if len(deals) > 0 {
		out.Trade = tradeFromDeals(id, deals, symbols, session)
	}
	return out
}

func orderFromDeals(orderID int64, deals []*pb.ProtoOADeal, positionID string, symbols helpers.Symbols, session *connector.Session) domain.FXOrder {
	id := strconv.FormatInt(orderID, 10)
	trade := tradeFromDeals(id, deals, symbols, session)
	symbolID := deals[0].GetSymbolId()
	return domain.FXOrder{
		ID:           id,
		PositionID:   positionID,
		Pair:         symbols.Pair(symbolID),
		Symbol:       symbols.Name(symbolID),
		Type:         fx.OrderMarket,
		Status:       fx.StatusFilled,
		Side:         trade.Side,
		Amount:       trade.Amount,
		AmountFilled: trade.Amount,
		AveragePrice: trade.Price,
		UpdatedAt:    trade.DoneAt,
		Trade:        trade,
	}
}

func tradeFromDeals(orderID string, deals []*pb.ProtoOADeal, symbols helpers.Symbols, session *connector.Session) domain.FXTrade {
	var volume int64
	var notional, commission, profit float64
	trade := domain.FXTrade{OrderID: orderID, Side: executionSide(deals[0].GetTradeSide())}
	for _, deal := range deals {
		v := filledVolume(deal)
		volume += v
		notional += float64(v) * deal.GetExecutionPrice()
		commission += dealCommission(deal, session)
		profit += dealProfit(deal, session)
		if at := helpers.TimeFromMillis(deal.GetExecutionTimestamp()); at.After(trade.DoneAt) {
			trade.DoneAt = at
		}
	}
	if volume > 0 {
		trade.Price = fx.Round8(notional / float64(volume))
	}
	trade.Amount = symbols.Lots(deals[0].GetSymbolId(), volume)
	trade.Commission = fx.Round8(commission)
	trade.Profit = fx.Round8(profit)
	return trade
}

func filledVolume(deal *pb.ProtoOADeal) int64 {
	if detail := deal.GetClosePositionDetail(); detail != nil && detail.GetClosedVolume() != 0 {
		return detail.GetClosedVolume()
	}
	if v := deal.GetFilledVolume(); v != 0 {
		return v
	}
	return deal.GetVolume()
}

func TradeSide(side pb.ProtoOATradeSide) string {
	if side == pb.ProtoOATradeSide_BUY {
		return fx.Long
	}
	return fx.Short
}

func executionSide(side pb.ProtoOATradeSide) string {
	if side == pb.ProtoOATradeSide_BUY {
		return fx.Buy
	}
	return fx.Sell
}

func orderType(order *pb.ProtoOAOrder) string {
	switch order.GetOrderType() {
	case pb.ProtoOAOrderType_MARKET, pb.ProtoOAOrderType_MARKET_RANGE:
		return fx.OrderMarket
	case pb.ProtoOAOrderType_LIMIT:
		return fx.OrderLimit
	case pb.ProtoOAOrderType_STOP:
		return fx.OrderStop
	case pb.ProtoOAOrderType_STOP_LIMIT:
		return "STOP_LIMIT"
	case pb.ProtoOAOrderType_STOP_LOSS_TAKE_PROFIT:
		if order.TakeProfit != nil && order.StopLoss == nil {
			return fx.OrderLimit
		}
		return fx.OrderStop
	default:
		return order.GetOrderType().String()
	}
}

func orderStatus(order *pb.ProtoOAOrder, filled bool) string {
	if filled || order.GetExecutedVolume() > 0 {
		if order.GetExecutedVolume() > 0 && order.GetExecutedVolume() < order.GetTradeData().GetVolume() && order.GetOrderStatus() != pb.ProtoOAOrderStatus_ORDER_STATUS_FILLED {
			return fx.StatusPartiallyFilled
		}
		return fx.StatusFilled
	}
	switch order.GetOrderStatus() {
	case pb.ProtoOAOrderStatus_ORDER_STATUS_FILLED:
		return fx.StatusFilled
	case pb.ProtoOAOrderStatus_ORDER_STATUS_ACCEPTED:
		return fx.StatusWorking
	case pb.ProtoOAOrderStatus_ORDER_STATUS_REJECTED:
		return "REJECTED"
	case pb.ProtoOAOrderStatus_ORDER_STATUS_EXPIRED:
		return "EXPIRED"
	case pb.ProtoOAOrderStatus_ORDER_STATUS_CANCELLED:
		return fx.StatusCanceled
	default:
		return order.GetOrderStatus().String()
	}
}

func dealCommission(deal *pb.ProtoOADeal, session *connector.Session) float64 {
	digits := helpers.MoneyDigits(session, deal)
	value := deal.GetCommission()
	if detail := deal.GetClosePositionDetail(); detail != nil && detail.GetCommission() != 0 {
		value = detail.GetCommission()
	}
	return fx.Round8(math.Abs(helpers.Money(value, digits)))
}

func dealProfit(deal *pb.ProtoOADeal, session *connector.Session) float64 {
	detail := deal.GetClosePositionDetail()
	if detail == nil {
		return 0
	}
	return fx.Round8(helpers.Money(detail.GetGrossProfit(), helpers.MoneyDigits(session, deal)))
}
