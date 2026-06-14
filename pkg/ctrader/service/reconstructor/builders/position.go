package builders

import (
	"sort"
	"strconv"

	"github.com/google/uuid"
	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func BuildFXPositions(
	deals []*pb.ProtoOADeal,
	orders []*pb.ProtoOAOrder,
	symbols map[int64]string,
	session *connector.Session,
) []domain.FXPosition {
	groupedDeals := make(map[int64][]*pb.ProtoOADeal)
	for _, deal := range deals {
		if deal == nil || deal.GetDealStatus() != pb.ProtoOADealStatus_FILLED {
			continue
		}
		groupedDeals[deal.GetPositionId()] = append(groupedDeals[deal.GetPositionId()], deal)
	}
	groupedOrders := make(map[int64][]*pb.ProtoOAOrder)
	for _, order := range orders {
		if order == nil {
			continue
		}
		groupedOrders[order.GetPositionId()] = append(groupedOrders[order.GetPositionId()], order)
	}

	positions := make([]domain.FXPosition, 0, len(groupedDeals))
	for positionID, positionDeals := range groupedDeals {
		sortDeals(positionDeals)
		if !hasCloseDeal(positionDeals) {
			continue
		}
		pos := buildFXPosition(positionID, positionDeals, groupedOrders[positionID], symbols, session)
		positions = append(positions, pos)
	}
	sort.Slice(positions, func(i, j int) bool { return positions[i].CreatedAt.Before(positions[j].CreatedAt) })
	return positions
}

func buildFXPosition(
	positionID int64,
	deals []*pb.ProtoOADeal,
	orders []*pb.ProtoOAOrder,
	symbols map[int64]string,
	session *connector.Session,
) domain.FXPosition {
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte("ctrader-position-"+strconv.FormatInt(positionID, 10)))
	opening := openingDeals(deals)
	closing := closingDeals(deals)
	first := deals[0]
	firstOpen := first
	if len(opening) > 0 {
		firstOpen = opening[0]
	}
	lastClose := closing[len(closing)-1]
	closedAt := helpers.TimeFromMillis(lastClose.GetExecutionTimestamp())

	pnl, swap, commission, pnlConversionFee := 0.0, 0.0, 0.0, 0.0
	balanceAfterClose := 0.0
	for _, deal := range closing {
		detail := deal.GetClosePositionDetail()
		digits := helpers.MoneyDigits(session, deal)
		pnl += helpers.Money(detail.GetGrossProfit(), digits)
		swap += helpers.Money(detail.GetSwap(), digits)
		commissionValue := detail.GetCommission()
		if commissionValue == 0 {
			commissionValue = deal.GetCommission()
		}
		commission += helpers.Abs(helpers.Money(commissionValue, digits))
		pnlConversionFee += helpers.Abs(helpers.Money(detail.GetPnlConversionFee(), digits))
		balanceAfterClose = helpers.Money(detail.GetBalance(), digits)
	}

	net := pnl + swap - commission - pnlConversionFee
	status := "lose"
	if net > 0 {
		status = "win"
	}
	amount := closedAmount(closing)
	entryPrice := closeEntryPrice(closing)
	if entryPrice == 0 {
		entryPrice = weightedPrice(opening)
	}
	positionOrders := BuildOrders(orders, deals, id, session)
	if len(positionOrders) == 0 {
		positionOrders = BuildOrdersFromDeals(deals, id, session)
	}
	tp, sl := extractProtection(orders, entryPrice, TradeSide(firstOpen.GetTradeSide()))
	return domain.FXPosition{
		ID:          id,
		Side:        TradeSide(firstOpen.GetTradeSide()),
		Pair:        helpers.SymbolName(symbols, first.GetSymbolId()),
		Amount:      amount,
		EntryPrice:  entryPrice,
		ExitPrice:   weightedPrice(closing),
		Pnl:         helpers.Round8(pnl),
		NetPnl:      helpers.Round8(net),
		Commission:  helpers.Round8(commission),
		Swap:        helpers.Round8(swap),
		TP:          tp,
		SL:          sl,
		Multiplier:  leverageMultiplier(session),
		Closed:      true,
		Status:      &status,
		CreatedAt:   helpers.TimeFromMillis(firstOpen.GetExecutionTimestamp()),
		ClosedAt:    &closedAt,
		Orders:      positionOrders,
		BalanceInit: helpers.Round8(balanceAfterClose - net),
	}
}

func extractProtection(orders []*pb.ProtoOAOrder, entryPrice float64, side string) (tp, sl *float64) {
	for _, order := range orders {
		if order == nil {
			continue
		}
		if order.TakeProfit != nil {
			v := order.GetTakeProfit()
			tp = &v
		}
		if order.StopLoss != nil {
			v := order.GetStopLoss()
			sl = &v
		}
		if tp == nil && order.RelativeTakeProfit != nil && entryPrice > 0 {
			v := relativeProtectionPrice(entryPrice, order.GetRelativeTakeProfit(), side, true)
			tp = &v
		}
		if sl == nil && order.RelativeStopLoss != nil && entryPrice > 0 {
			v := relativeProtectionPrice(entryPrice, order.GetRelativeStopLoss(), side, false)
			sl = &v
		}
	}
	return tp, sl
}

func relativeProtectionPrice(entryPrice float64, rawDistance int64, side string, takeProfit bool) float64 {
	distance := float64(rawDistance) / 100000.0
	if side == "LONG" {
		if takeProfit {
			return helpers.Round8(entryPrice + distance)
		}
		return helpers.Round8(entryPrice - distance)
	}
	if takeProfit {
		return helpers.Round8(entryPrice - distance)
	}
	return helpers.Round8(entryPrice + distance)
}

func leverageMultiplier(session *connector.Session) uint32 {
	if session == nil || session.LeverageInCents == 0 {
		return 1
	}
	multiplier := session.LeverageInCents / 100
	if multiplier == 0 {
		return 1
	}
	return multiplier
}

func hasCloseDeal(deals []*pb.ProtoOADeal) bool {
	for _, deal := range deals {
		if deal.GetClosePositionDetail() != nil {
			return true
		}
	}
	return false
}

func openingDeals(deals []*pb.ProtoOADeal) []*pb.ProtoOADeal {
	out := make([]*pb.ProtoOADeal, 0, len(deals))
	for _, deal := range deals {
		if deal.GetClosePositionDetail() == nil {
			out = append(out, deal)
		}
	}
	return out
}

func closingDeals(deals []*pb.ProtoOADeal) []*pb.ProtoOADeal {
	out := make([]*pb.ProtoOADeal, 0, len(deals))
	for _, deal := range deals {
		if deal.GetClosePositionDetail() != nil {
			out = append(out, deal)
		}
	}
	return out
}

func sortDeals(deals []*pb.ProtoOADeal) {
	sort.Slice(deals, func(i, j int) bool { return deals[i].GetExecutionTimestamp() < deals[j].GetExecutionTimestamp() })
}

func closedAmount(deals []*pb.ProtoOADeal) float64 {
	var volume int64
	for _, deal := range deals {
		if detail := deal.GetClosePositionDetail(); detail != nil && detail.GetClosedVolume() != 0 {
			volume += detail.GetClosedVolume()
			continue
		}
		volume += deal.GetFilledVolume()
	}
	return volumeToAmount(volume)
}

func closeEntryPrice(closing []*pb.ProtoOADeal) float64 {
	for _, deal := range closing {
		if detail := deal.GetClosePositionDetail(); detail != nil && detail.GetEntryPrice() != 0 {
			return detail.GetEntryPrice()
		}
	}
	return 0
}

func weightedPrice(deals []*pb.ProtoOADeal) float64 {
	var volume int64
	var weighted float64
	for _, deal := range deals {
		v := deal.GetFilledVolume()
		if v == 0 {
			v = deal.GetVolume()
		}
		volume += v
		weighted += float64(v) * deal.GetExecutionPrice()
	}
	if volume == 0 {
		return 0
	}
	return weighted / float64(volume)
}
