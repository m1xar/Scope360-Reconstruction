package builders

import (
	"math"
	"strconv"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

func BuildOpenPositions(
	res *pb.ProtoOAReconcileRes,
	deals []*pb.ProtoOADeal,
	unrealized *pb.ProtoOAGetPositionUnrealizedPnLRes,
	prices map[int64]helpers.Quote,
	symbols helpers.Symbols,
	session *connector.Session,
) []domain.FXOpenPosition {
	if res == nil {
		return []domain.FXOpenPosition{}
	}
	ordersByPosition := make(map[int64][]*pb.ProtoOAOrder)
	for _, order := range res.GetOrder() {
		if order.GetClosingOrder() {
			ordersByPosition[order.GetPositionId()] = append(ordersByPosition[order.GetPositionId()], order)
		}
	}
	dealsByPosition := make(map[int64][]*pb.ProtoOADeal)
	for _, deal := range deals {
		if deal.GetDealStatus() == pb.ProtoOADealStatus_FILLED {
			dealsByPosition[deal.GetPositionId()] = append(dealsByPosition[deal.GetPositionId()], deal)
		}
	}
	upl := make(map[int64]float64)
	if unrealized != nil {
		for _, item := range unrealized.GetPositionUnrealizedPnL() {
			upl[item.GetPositionId()] = helpers.Money(item.GetGrossUnrealizedPnL(), unrealized.GetMoneyDigits())
		}
	}

	out := make([]domain.FXOpenPosition, 0, len(res.GetPosition()))
	for _, pos := range res.GetPosition() {
		tradeData := pos.GetTradeData()
		symbolID := tradeData.GetSymbolId()
		positionID := pos.GetPositionId()
		id := strconv.FormatInt(positionID, 10)
		side := TradeSide(tradeData.GetTradeSide())
		amount := symbols.Lots(symbolID, tradeData.GetVolume())
		openedAt := helpers.TimeFromMillis(tradeData.GetOpenTimestamp())
		digits := positionMoneyDigits(pos, session)
		tp, sl := fx.PriceOrNil(pos.GetTakeProfit()), fx.PriceOrNil(pos.GetStopLoss())

		positionDeals := dealsByPosition[positionID]
		sortDeals(positionDeals)
		opening, closing := splitDeals(positionDeals)
		var orders []domain.FXOrder
		if len(opening) == 0 {
			orders = append(orders, syntheticOpening(id, symbols.Pair(symbolID), symbols.Name(symbolID), side, amount, pos.GetPrice(), openedAt, tp, sl))
		}
		orders = append(orders, BuildOrders(nil, append(append([]*pb.ProtoOADeal{}, opening...), closing...), id, tp, sl, symbols, session)...)
		orders = append(orders, BuildOrders(ordersByPosition[positionID], nil, id, tp, sl, symbols, session)...)
		for i := range orders {
			if orders[i].Status == fx.StatusWorking && orders[i].Amount == 0 {
				orders[i].Amount = amount
			}
		}

		out = append(out, domain.FXOpenPosition{
			ID:            id,
			Pair:          symbols.Pair(symbolID),
			Symbol:        symbols.Name(symbolID),
			Amount:        amount,
			Side:          side,
			EntryPrice:    pos.GetPrice(),
			CurrentPrice:  currentPrice(side, prices[symbolID]),
			UnrealizedPnl: fx.Round8(upl[positionID]),
			Commission:    fx.Round8(math.Abs(helpers.Money(pos.GetCommission(), digits))),
			Swap:          fx.Round8(helpers.Money(pos.GetSwap(), digits)),
			Margin:        fx.Round8(helpers.Money(int64(pos.GetUsedMargin()), digits)),
			OpenTime:      openedAt,
			Orders:        orders,
		})
	}
	return out
}

func syntheticOpening(id, pair, symbol, side string, amount, price float64, at time.Time, tp, sl *float64) domain.FXOrder {
	orderSide := fx.OrderSide(side)
	return domain.FXOrder{
		ID: id, PositionID: id, Pair: pair, Symbol: symbol, Type: fx.OrderMarket, Status: fx.StatusFilled, Side: orderSide,
		Amount: amount, AmountFilled: amount, AveragePrice: price, TP: tp, SL: sl, UpdatedAt: at,
		Trade: domain.FXTrade{OrderID: id, Side: orderSide, Price: price, Amount: amount, DoneAt: at},
	}
}

func currentPrice(side string, quote helpers.Quote) float64 {
	if side == fx.Short {
		if quote.Ask > 0 {
			return quote.Ask
		}
		return quote.Bid
	}
	if quote.Bid > 0 {
		return quote.Bid
	}
	return quote.Ask
}

func positionMoneyDigits(pos *pb.ProtoOAPosition, session *connector.Session) uint32 {
	if pos.MoneyDigits != nil {
		return pos.GetMoneyDigits()
	}
	if session != nil {
		return session.MoneyDigits
	}
	return 0
}
