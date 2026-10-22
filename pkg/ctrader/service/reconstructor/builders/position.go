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

func BuildFXPositions(
	deals []*pb.ProtoOADeal,
	orders []*pb.ProtoOAOrder,
	openIDs map[int64]bool,
	symbols helpers.Symbols,
	session *connector.Session,
) []domain.FXPosition {
	groupedDeals := make(map[int64][]*pb.ProtoOADeal)
	for _, deal := range deals {
		if deal.GetDealStatus() == pb.ProtoOADealStatus_FILLED {
			groupedDeals[deal.GetPositionId()] = append(groupedDeals[deal.GetPositionId()], deal)
		}
	}
	groupedOrders := make(map[int64][]*pb.ProtoOAOrder)
	for _, order := range orders {
		groupedOrders[order.GetPositionId()] = append(groupedOrders[order.GetPositionId()], order)
	}

	positions := make([]domain.FXPosition, 0, len(groupedDeals))
	for positionID, positionDeals := range groupedDeals {
		sortDeals(positionDeals)
		opening, closing := splitDeals(positionDeals)
		if len(closing) == 0 {
			continue
		}
		closed := !openIDs[positionID] && (len(opening) == 0 || closedVolume(closing) >= filledTotal(opening))
		positions = append(positions, buildFXPosition(positionID, opening, closing, groupedOrders[positionID], symbols, session, closed))
	}
	fx.SortPositions(positions)
	return positions
}

func buildFXPosition(
	positionID int64,
	opening, closing []*pb.ProtoOADeal,
	orders []*pb.ProtoOAOrder,
	symbols helpers.Symbols,
	session *connector.Session,
	closed bool,
) domain.FXPosition {
	id := strconv.FormatInt(positionID, 10)
	if !closed {
		id += ":partial"
	}
	first := closing[0]
	if len(opening) > 0 {
		first = opening[0]
	}
	lastClose := closing[len(closing)-1]
	closedAt := helpers.TimeFromMillis(lastClose.GetExecutionTimestamp())
	symbolID := first.GetSymbolId()
	side := TradeSide(first.GetTradeSide())

	var pnl, swap, commission, conversionFee, balanceAfterClose float64
	for _, deal := range closing {
		detail := deal.GetClosePositionDetail()
		digits := helpers.MoneyDigits(session, deal)
		pnl += helpers.Money(detail.GetGrossProfit(), digits)
		swap += helpers.Money(detail.GetSwap(), digits)
		commission += dealCommission(deal, session)
		conversionFee += math.Abs(helpers.Money(detail.GetPnlConversionFee(), digits))
		balanceAfterClose = helpers.Money(detail.GetBalance(), digits)
	}
	net := pnl + swap - commission - conversionFee

	entryPrice := closing[0].GetClosePositionDetail().GetEntryPrice()
	if entryPrice == 0 {
		entryPrice = weightedPrice(opening)
	}
	tp, sl := extractProtection(orders, entryPrice, side)
	pos := domain.FXPosition{
		ID:           id,
		Side:         side,
		Pair:         symbols.Pair(symbolID),
		Symbol:       symbols.Name(symbolID),
		Amount:       symbols.Lots(symbolID, closedVolume(closing)),
		EntryPrice:   entryPrice,
		ExitPrice:    weightedPrice(closing),
		Pnl:          fx.Round8(pnl),
		NetPnl:       fx.Round8(net),
		Commission:   fx.Round8(commission),
		Swap:         fx.Round8(swap),
		TP:           tp,
		SL:           sl,
		Multiplier:   leverageMultiplier(session),
		Closed:       closed,
		CloseReason:  closeReason(lastClose, orders),
		Status:       fx.Status(net),
		CreatedAt:    helpers.TimeFromMillis(first.GetExecutionTimestamp()),
		ClosedAt:     &closedAt,
		BalanceInit:  fx.Round8(balanceAfterClose - net),
		ContractSize: symbols.ContractSize(symbolID),
	}
	fx.ApplyRR(&pos)
	pos.Orders = BuildOrders(orders, append(append([]*pb.ProtoOADeal{}, opening...), closing...), id, tp, sl, symbols, session)
	return pos
}

func closeReason(deal *pb.ProtoOADeal, orders []*pb.ProtoOAOrder) string {
	for _, order := range orders {
		if order.GetOrderId() != deal.GetOrderId() {
			continue
		}
		switch {
		case order.GetIsStopOut():
			return "STOP_OUT"
		case order.GetOrderType() != pb.ProtoOAOrderType_STOP_LOSS_TAKE_PROFIT:
			return "USER"
		case order.StopLoss != nil:
			return "SL"
		case order.TakeProfit != nil:
			return "TP"
		}
	}
	return "USER"
}

func extractProtection(orders []*pb.ProtoOAOrder, entryPrice float64, side string) (tp, sl *float64) {
	for _, order := range orders {
		if order.TakeProfit != nil {
			tp = fx.PriceOrNil(order.GetTakeProfit())
		}
		if order.StopLoss != nil {
			sl = fx.PriceOrNil(order.GetStopLoss())
		}
		if tp == nil && order.RelativeTakeProfit != nil && entryPrice > 0 {
			tp = fx.PriceOrNil(relativeProtectionPrice(entryPrice, order.GetRelativeTakeProfit(), side, true))
		}
		if sl == nil && order.RelativeStopLoss != nil && entryPrice > 0 {
			sl = fx.PriceOrNil(relativeProtectionPrice(entryPrice, order.GetRelativeStopLoss(), side, false))
		}
	}
	return tp, sl
}

func relativeProtectionPrice(entryPrice float64, rawDistance int64, side string, takeProfit bool) float64 {
	distance := float64(rawDistance) / 100000.0
	if (side == fx.Long) == takeProfit {
		return fx.Round8(entryPrice + distance)
	}
	return fx.Round8(entryPrice - distance)
}

func leverageMultiplier(session *connector.Session) uint32 {
	if session == nil || session.LeverageInCents < 100 {
		return 1
	}
	return session.LeverageInCents / 100
}

func splitDeals(deals []*pb.ProtoOADeal) (opening, closing []*pb.ProtoOADeal) {
	for _, deal := range deals {
		if deal.GetClosePositionDetail() != nil {
			closing = append(closing, deal)
		} else {
			opening = append(opening, deal)
		}
	}
	return opening, closing
}

func sortDeals(deals []*pb.ProtoOADeal) {
	sort.SliceStable(deals, func(i, j int) bool { return deals[i].GetExecutionTimestamp() < deals[j].GetExecutionTimestamp() })
}

func closedVolume(closing []*pb.ProtoOADeal) int64 {
	var volume int64
	for _, deal := range closing {
		volume += filledVolume(deal)
	}
	return volume
}

func filledTotal(deals []*pb.ProtoOADeal) int64 {
	var volume int64
	for _, deal := range deals {
		volume += filledVolume(deal)
	}
	return volume
}

func weightedPrice(deals []*pb.ProtoOADeal) float64 {
	var volume int64
	var weighted float64
	for _, deal := range deals {
		v := filledVolume(deal)
		volume += v
		weighted += float64(v) * deal.GetExecutionPrice()
	}
	if volume == 0 {
		return 0
	}
	return fx.Round8(weighted / float64(volume))
}
