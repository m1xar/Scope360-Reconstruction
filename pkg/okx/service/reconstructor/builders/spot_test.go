package builders

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
)

var spotTestStart = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

var spotTestNow = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

func buildSpotTest(legs []helpers.SpotLeg, holdings []helpers.SpotHolding, windowStart time.Time, priceAt helpers.PriceAt, minSizes map[string]float64, adjustments []helpers.SpotAdjustment) ([]domain.Position, []domain.OpenPosition) {
	return BuildSpotPositions(legs, holdings, windowStart, spotTestNow, priceAt, minSizes, adjustments)
}

func spotTestLeg(asset, side string, qty, px float64, day int) helpers.SpotLeg {
	return helpers.SpotLeg{
		Asset:   asset,
		Quote:   "USDT",
		InstId:  asset + "-USDT",
		OrdId:   asset + side + time.Duration(day).String(),
		OrdType: "market",
		Side:    side,
		Qty:     qty,
		PxUSD:   px,
		FeeUSD:  0.01,
		Ts:      spotTestStart.AddDate(0, 0, day).UnixMilli(),
	}
}

func spotNear(a, b float64) bool {
	return math.Abs(a-b) <= 1e-8*math.Max(1, math.Abs(b))
}

func spotTestDay(day int) int64 {
	return spotTestStart.AddDate(0, 0, day).UnixMilli()
}

func spotTestPrices(prices map[int]float64) helpers.PriceAt {
	return func(asset string, ts int64) (float64, bool) {
		px, ok := prices[int((helpers.SpotDayStart(ts)-spotTestStart.UnixMilli())/helpers.SpotDayMs)]
		return px, ok
	}
}

func spotAdjustmentOrders(orders []domain.Order) []domain.Order {
	var out []domain.Order
	for _, order := range orders {
		if order.Status == helpers.OrderStatusAdjustment {
			out = append(out, order)
		}
	}
	return out
}

func spotOpeningSynthetics(orders []domain.Order) int {
	n := 0
	for _, order := range orders {
		if order.Status == helpers.OrderStatusSynthetic && strings.HasPrefix(order.ExchangeOrderID, "synthetic:") {
			n++
		}
	}
	return n
}

func TestSpotNegativeAdjustmentIsSyntheticSellOnItsDay(t *testing.T) {
	legs := []helpers.SpotLeg{spotTestLeg("BTC", "buy", 0.05, 60000, 1)}
	holdings := []helpers.SpotHolding{{Asset: "BTC", Qty: 0.0185, AvgPx: 60000, PriceUSD: 65000}}
	adjustments := []helpers.SpotAdjustment{{Asset: "BTC", Qty: -0.0315, Ts: spotTestDay(2)}}
	prices := spotTestPrices(map[int]float64{2: 62000})

	closed, open := buildSpotTest(legs, holdings, spotTestStart, prices, map[string]float64{"BTC": 0.00001}, adjustments)
	if len(closed) != 0 || len(open) != 1 {
		t.Fatalf("want one open BTC position, got %d closed %d open", len(closed), len(open))
	}
	pos := open[0]
	if !spotNear(pos.Amount, 0.0185) || !spotNear(pos.EntryPrice, 60000) {
		t.Fatalf("open %v @ %v, want 0.0185 @ 60000", pos.Amount, pos.EntryPrice)
	}
	if spotOpeningSynthetics(pos.Orders) != 0 || len(pos.Orders) != 2 {
		t.Fatalf("want the buy and the synthetic sell only, got %+v", pos.Orders)
	}
	adj := spotAdjustmentOrders(pos.Orders)
	if len(adj) != 1 {
		t.Fatalf("want one synthetic adjustment order, got %d", len(adj))
	}
	sell := adj[0]
	wantAt := time.UnixMilli(spotTestDay(3) - 1).UTC()
	if sell.Side != "SELL" || !spotNear(sell.AmountFilled, 0.0315) || !spotNear(sell.AveragePrice, 62000) || !sell.Trade.DoneAt.Equal(wantAt) {
		t.Fatalf("unexpected synthetic sell %+v", sell)
	}
	if sell.ExchangeOrderID != adjustments[0].Key() || sell.Trade.Commission != 0 {
		t.Fatalf("synthetic sell must carry the adjustment key and no fee, got %q %v", sell.ExchangeOrderID, sell.Trade.Commission)
	}
	if !spotNear(sell.Trade.Profit, 63) || !spotNear(pos.RealizedPnl, 63) {
		t.Fatalf("realized %v / %v, want 63 against the 60000 VWAP", sell.Trade.Profit, pos.RealizedPnl)
	}
}

func TestSpotPositiveAdjustmentIsSyntheticBuyAtDayPrice(t *testing.T) {
	legs := []helpers.SpotLeg{spotTestLeg("BTC", "buy", 0.05, 60000, 1)}
	holdings := []helpers.SpotHolding{{Asset: "BTC", Qty: 0.055, PriceUSD: 65000}}
	adjustments := []helpers.SpotAdjustment{{Asset: "BTC", Qty: 0.005, Ts: spotTestDay(2)}}
	prices := spotTestPrices(map[int]float64{2: 62000})

	_, open := buildSpotTest(legs, holdings, spotTestStart, prices, map[string]float64{"BTC": 0.00001}, adjustments)
	if len(open) != 1 || !spotNear(open[0].Amount, 0.055) {
		t.Fatalf("want open 0.055 BTC, got %+v", open)
	}
	if want := (0.05*60000 + 0.005*62000) / 0.055; !spotNear(open[0].EntryPrice, want) {
		t.Fatalf("entry %v, want %v", open[0].EntryPrice, want)
	}
	adj := spotAdjustmentOrders(open[0].Orders)
	if len(adj) != 1 || adj[0].Side != "BUY" || !adj[0].Trade.DoneAt.Equal(spotTestStart.AddDate(0, 0, 2)) {
		t.Fatalf("want one synthetic buy at the day start, got %+v", adj)
	}
	if spotOpeningSynthetics(open[0].Orders) != 0 {
		t.Fatalf("no opening synthetic expected")
	}
}

func TestSpotNegativeAdjustmentClosesLotWhenBalanceIsGone(t *testing.T) {
	legs := []helpers.SpotLeg{
		spotTestLeg("ETH", "buy", 0.003, 2500, 1),
		spotTestLeg("ETH", "sell", 0.0008, 2700, 3),
	}
	holdings := []helpers.SpotHolding{{Asset: "ETH", Qty: 0, PriceUSD: 2750}}
	adjustments := []helpers.SpotAdjustment{{Asset: "ETH", Qty: -0.0022, Ts: spotTestDay(4)}}
	prices := spotTestPrices(map[int]float64{4: 2600})

	closed, open := buildSpotTest(legs, holdings, spotTestStart, prices, map[string]float64{"ETH": 0.0001}, adjustments)
	if len(open) != 0 || len(closed) != 1 {
		t.Fatalf("want one closed ETH sale, got %d closed %d open", len(closed), len(open))
	}
	sale := closed[0]
	if !spotNear(sale.Amount, 0.003) || !spotNear(sale.EntryPrice, 2500) || !spotNear(sale.Pnl, 0.38) || len(sale.Orders) != 3 {
		t.Fatalf("unexpected sale %+v", sale)
	}
	if !sale.ClosedAt.Equal(time.UnixMilli(spotTestDay(5) - 1).UTC()) {
		t.Fatalf("closed at the synthetic sell, got %v", sale.ClosedAt)
	}
}

func TestSpotSameDayNegativeAdjustmentFollowsTheBuy(t *testing.T) {
	buy := spotTestLeg("BTC", "buy", 0.1, 60000, 5)
	buy.Ts += (12 * time.Hour).Milliseconds()
	holdings := []helpers.SpotHolding{{Asset: "BTC", Qty: 0.05, PriceUSD: 61000}}
	adjustments := []helpers.SpotAdjustment{{Asset: "BTC", Qty: -0.05, Ts: spotTestDay(5)}}
	prices := spotTestPrices(map[int]float64{5: 60500})
	minSizes := map[string]float64{"BTC": 0.00001}

	closed, open := buildSpotTest([]helpers.SpotLeg{buy}, holdings, spotTestStart, prices, minSizes, adjustments)
	if len(closed) != 0 || len(open) != 1 {
		t.Fatalf("want a single open position, got %d closed %d open", len(closed), len(open))
	}
	pos := open[0]
	if !spotNear(pos.Amount, 0.05) || spotOpeningSynthetics(pos.Orders) != 0 || len(pos.Orders) != 2 {
		t.Fatalf("want open 0.05 from the buy and one synthetic sell, got %+v", pos)
	}
	sell := pos.Orders[1]
	if sell.Status != helpers.OrderStatusAdjustment || sell.Side != "SELL" || !sell.Trade.DoneAt.After(pos.Orders[0].Trade.DoneAt) {
		t.Fatalf("synthetic sell must follow the 12:00 buy, got %+v", pos.Orders)
	}
	if !spotNear(sell.Trade.Profit, 25) {
		t.Fatalf("realized %v, want 25", sell.Trade.Profit)
	}
}

func TestSpotTransfersAndOffsettingAdjustmentsCloseWithoutOpeningSynthetic(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	d0909 := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	d0922 := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	transfer := func(side string, qty, px float64, at time.Time, id string) helpers.SpotLeg {
		return helpers.SpotLeg{Asset: "ETH", Quote: "USDT", InstId: "ETH-USDT", OrdId: id, OrdType: "market", Side: side, Qty: qty, PxUSD: px, Ts: at.UnixMilli(), Transfer: true}
	}
	legs := []helpers.SpotLeg{
		transfer("buy", 0.00432653, 4300, d0909.Add(10*time.Hour), "bill-in"),
		transfer("sell", 0.00223219, 4500, d0922.Add(14*time.Hour), "bill-out"),
	}
	adjustments := []helpers.SpotAdjustment{
		{Asset: "ETH", Qty: -0.00432653, Ts: d0909.UnixMilli()},
		{Asset: "ETH", Qty: 0.0022321950014775, Ts: d0922.UnixMilli()},
	}
	prices := func(asset string, ts int64) (float64, bool) {
		switch helpers.SpotDayStart(ts) {
		case d0909.UnixMilli():
			return 4300, true
		case d0922.UnixMilli():
			return 4500, true
		}
		return 0, false
	}

	closed, open := buildSpotTest(legs, nil, start, prices, map[string]float64{"ETH": 0.0001}, adjustments)
	if len(open) != 0 || len(closed) != 1 {
		t.Fatalf("want one closed ETH position, got %d closed %d open", len(closed), len(open))
	}
	sale := closed[0]
	if spotOpeningSynthetics(sale.Orders) != 0 || len(sale.Orders) != 4 {
		t.Fatalf("want 4 executions and no window-start synthetic, got %+v", sale.Orders)
	}
	want := []struct {
		side, status string
		qty          float64
	}{
		{"BUY", helpers.OrderStatusTransfer, 0.00432653},
		{"SELL", helpers.OrderStatusAdjustment, 0.00432653},
		{"BUY", helpers.OrderStatusAdjustment, 0.0022322},
		{"SELL", helpers.OrderStatusTransfer, 0.00223219},
	}
	for i, w := range want {
		o := sale.Orders[i]
		if o.Side != w.side || o.Status != w.status || !spotNear(o.AmountFilled, w.qty) {
			t.Fatalf("execution %d: got %s %s %v, want %s %s %v", i, o.Side, o.Status, o.AmountFilled, w.side, w.status, w.qty)
		}
	}
	if !spotNear(sale.Orders[1].Trade.Profit, 0) || !spotNear(sale.Orders[2].AveragePrice, 4500) {
		t.Fatalf("synthetic sell pnl %v, synthetic buy price %v", sale.Orders[1].Trade.Profit, sale.Orders[2].AveragePrice)
	}
	if !spotNear(sale.Pnl, 0) {
		t.Fatalf("pnl %v, want 0", sale.Pnl)
	}
}

func TestSpotAdjustmentBuyBetweenTradesJoinsTheSale(t *testing.T) {
	legs := []helpers.SpotLeg{
		spotTestLeg("SOL", "buy", 0.1, 100, 1),
		spotTestLeg("SOL", "sell", 0.3, 130, 3),
	}
	adjustments := []helpers.SpotAdjustment{{Asset: "SOL", Qty: 0.2, Ts: spotTestDay(2)}}
	prices := spotTestPrices(map[int]float64{2: 120})
	holdings := []helpers.SpotHolding{{Asset: "SOL", Qty: 0, PriceUSD: 140}}

	closed, open := buildSpotTest(legs, holdings, spotTestStart, prices, map[string]float64{"SOL": 0.001}, adjustments)
	if len(open) != 0 || len(closed) != 1 {
		t.Fatalf("want one closed SOL position, got %d closed %d open", len(closed), len(open))
	}
	sale := closed[0]
	if len(sale.Orders) != 3 || spotOpeningSynthetics(sale.Orders) != 0 {
		t.Fatalf("want buy, synthetic buy, sell; got %+v", sale.Orders)
	}
	syn := sale.Orders[1]
	if syn.Status != helpers.OrderStatusAdjustment || syn.Side != "BUY" || syn.ExchangeOrderID != adjustments[0].Key() || !spotNear(syn.AmountFilled, 0.2) || !spotNear(syn.AveragePrice, 120) || !syn.Trade.DoneAt.Equal(spotTestStart.AddDate(0, 0, 2)) {
		t.Fatalf("unexpected synthetic buy %+v", syn)
	}
	if !spotNear(sale.Amount, 0.3) || !spotNear(sale.EntryPrice, 34.0/0.3) || !spotNear(sale.ExitPrice, 130) || !spotNear(sale.Pnl, 5) {
		t.Fatalf("unexpected sale %v @ %v -> %v pnl %v", sale.Amount, sale.EntryPrice, sale.ExitPrice, sale.Pnl)
	}
}

func TestSpotUnexplainedRemainderStillOpensAtWindowStart(t *testing.T) {
	legs := []helpers.SpotLeg{spotTestLeg("BTC", "sell", 0.02, 61000, 3)}
	holdings := []helpers.SpotHolding{{Asset: "BTC", Qty: 0.05, AvgPx: 50000, PriceUSD: 65000}}
	adjustments := []helpers.SpotAdjustment{{Asset: "BTC", Qty: 0.01, Ts: spotTestDay(2)}}
	prices := spotTestPrices(map[int]float64{2: 60000})

	_, open := buildSpotTest(legs, holdings, spotTestStart, prices, map[string]float64{"BTC": 0.00001}, adjustments)
	if len(open) != 1 || !spotNear(open[0].Amount, 0.05) {
		t.Fatalf("want open 0.05, got %+v", open)
	}
	first := open[0].Orders[0]
	if first.Status != helpers.OrderStatusSynthetic || first.ExchangeOrderID != helpers.SpotSyntheticKey("BTC", spotTestStart.UnixMilli()) || !spotNear(first.AmountFilled, 0.06) || !first.Trade.DoneAt.Equal(spotTestStart) {
		t.Fatalf("want a 0.06 opening synthetic at the window start, got %+v", first)
	}
}

func spotTwoLots() []helpers.SpotLeg {
	return []helpers.SpotLeg{
		spotTestLeg("BTC", "buy", 1, 100, 1),
		spotTestLeg("BTC", "sell", 1, 110, 2),
		spotTestLeg("BTC", "buy", 2, 200, 3),
	}
}

func TestSpotAssetGroupsClosedLotsIntoOneSale(t *testing.T) {
	legs := append(spotTwoLots(), spotTestLeg("BTC", "sell", 2, 190, 4))
	holdings := []helpers.SpotHolding{{Asset: "BTC", Qty: 0, PriceUSD: 195}}
	minSizes := map[string]float64{"BTC": 0.00001}

	closed, open := buildSpotTest(legs, holdings, spotTestStart, nil, minSizes, nil)
	if len(open) != 0 || len(closed) != 1 {
		t.Fatalf("want one closed sale, got %d closed %d open", len(closed), len(open))
	}
	sale := closed[0]
	if !spotNear(sale.Amount, 3) || !spotNear(sale.Pnl, -10) || !spotNear(sale.Commission, 0.04) || !spotNear(sale.NetPnl, -10.04) {
		t.Fatalf("unexpected totals %+v", sale)
	}
	if !spotNear(sale.ExitPrice, 490.0/3) || !spotNear(sale.EntryPrice, 500.0/3) {
		t.Fatalf("unexpected vwap entry %v exit %v", sale.EntryPrice, sale.ExitPrice)
	}
	if !sale.CreatedAt.Equal(spotTestStart.AddDate(0, 0, 1)) || !sale.ClosedAt.Equal(spotTestStart.AddDate(0, 0, 4)) {
		t.Fatalf("want first buy to last sell, got %v - %v", sale.CreatedAt, sale.ClosedAt)
	}
	if len(sale.Orders) != 4 || *sale.Status != "lose" {
		t.Fatalf("want 4 executions and a loss, got %d %s", len(sale.Orders), *sale.Status)
	}
	var sellProfit float64
	for i, order := range sale.Orders {
		if order.PositionID != sale.ID {
			t.Fatalf("order %d belongs to another position", i)
		}
		if i > 0 && order.Trade.DoneAt.Before(sale.Orders[i-1].Trade.DoneAt) {
			t.Fatalf("executions are not in time order")
		}
		sellProfit += order.Trade.Profit
	}
	if !spotNear(sellProfit, -10.02) {
		t.Fatalf("sell profits must sum to the net of both lots (10 - 20 - fees), got %v", sellProfit)
	}
}

func TestSpotAssetGroupsHeldAssetIntoOneOpenPosition(t *testing.T) {
	legs := append(spotTwoLots(), spotTestLeg("BTC", "sell", 1, 220, 4))
	holdings := []helpers.SpotHolding{{Asset: "BTC", Qty: 1, PriceUSD: 230}}
	minSizes := map[string]float64{"BTC": 0.00001}

	closed, open := buildSpotTest(legs, holdings, spotTestStart, nil, minSizes, nil)
	if len(closed) != 0 || len(open) != 1 {
		t.Fatalf("want one open position, got %d closed %d open", len(closed), len(open))
	}
	pos := open[0]
	if !spotNear(pos.Amount, 1) || !spotNear(pos.EntryPrice, 200) || !spotNear(pos.CurrentPrice, 230) {
		t.Fatalf("open balance must keep the live lot basis, got %+v", pos)
	}
	if !spotNear(pos.RealizedPnl, 30) || !spotNear(pos.Commission, 0.04) {
		t.Fatalf("realized %v commission %v, want 30 and 0.04", pos.RealizedPnl, pos.Commission)
	}
	if !pos.OpenTime.Equal(spotTestStart.AddDate(0, 0, 1)) || len(pos.Orders) != 4 {
		t.Fatalf("want all 4 executions from the first buy, got %d from %v", len(pos.Orders), pos.OpenTime)
	}
	if pos.Symbol != "BTC-USDT" || pos.Pair != "BTCUSDT" {
		t.Fatalf("unexpected market %s %s", pos.Symbol, pos.Pair)
	}
}

func TestSpotAssetSyntheticHoldingWithoutHistory(t *testing.T) {
	holdings := []helpers.SpotHolding{{Asset: "SOL", Qty: 3, AvgPx: 150, PriceUSD: 160}}
	closed, open := buildSpotTest(nil, holdings, spotTestStart, nil, map[string]float64{"SOL": 0.001}, nil)
	if len(closed) != 0 || len(open) != 1 || !spotNear(open[0].Amount, 3) || !spotNear(open[0].EntryPrice, 150) {
		t.Fatalf("want one synthetic SOL holding, got %+v", open)
	}
	if len(open[0].Orders) != 1 || open[0].Orders[0].Status != helpers.OrderStatusSynthetic {
		t.Fatalf("want one synthetic opening order")
	}
}

func TestSpotAssetAdjustmentSellClosesTheLastLot(t *testing.T) {
	legs := []helpers.SpotLeg{
		spotTestLeg("ETH", "buy", 1, 2000, 1),
		spotTestLeg("ETH", "sell", 1, 2100, 2),
		spotTestLeg("ETH", "buy", 0.5, 2200, 3),
	}
	holdings := []helpers.SpotHolding{{Asset: "ETH", Qty: 0, PriceUSD: 2300}}
	adjustments := []helpers.SpotAdjustment{{Asset: "ETH", Qty: -0.5, Ts: spotTestDay(4)}}
	prices := spotTestPrices(map[int]float64{4: 2250})
	minSizes := map[string]float64{"ETH": 0.0001}

	closed, open := buildSpotTest(legs, holdings, spotTestStart, prices, minSizes, adjustments)
	if len(open) != 0 || len(closed) != 1 {
		t.Fatalf("want one closed sale, got %d closed %d open", len(closed), len(open))
	}
	sale := closed[0]
	if len(sale.Orders) != 4 || !spotNear(sale.Pnl, 125) || !spotNear(sale.Amount, 1.5) || !spotNear(sale.Commission, 0.03) {
		t.Fatalf("want 4 executions with pnl 125 on 1.5 sold, got %d %v %v %v", len(sale.Orders), sale.Pnl, sale.Amount, sale.Commission)
	}
	if !sale.ClosedAt.Equal(time.UnixMilli(spotTestDay(5) - 1).UTC()) {
		t.Fatalf("closed at the synthetic sell, got %v", sale.ClosedAt)
	}
	if sell := sale.Orders[3]; sell.Status != helpers.OrderStatusAdjustment || !spotNear(sell.Trade.Profit, 25) {
		t.Fatalf("the adjustment sell must realize 25 against the second lot, got %+v", sell)
	}
}

func TestSpotMidHistoryShortfallIsPricedAtTheSell(t *testing.T) {
	legs := []helpers.SpotLeg{
		spotTestLeg("BTC", "sell", 0.02, 150, 2),
		spotTestLeg("BTC", "buy", 0.02, 160, 3),
	}
	prices := spotTestPrices(map[int]float64{0: 50, 2: 151})

	closed, open := buildSpotTest(legs, nil, spotTestStart, prices, map[string]float64{"BTC": 0.00001}, nil)
	if len(open) != 0 || len(closed) != 1 {
		t.Fatalf("want one closed position, got %d closed %d open", len(closed), len(open))
	}
	sale := closed[0]
	syn := sale.Orders[0]
	if syn.Status != helpers.OrderStatusSynthetic || syn.ExchangeOrderID != helpers.SpotSyntheticKey("BTC", spotTestDay(2)) || !spotNear(syn.AveragePrice, 150) || !syn.Trade.DoneAt.Equal(spotTestStart.AddDate(0, 0, 2)) {
		t.Fatalf("want the shortfall bought at the sell price and time, got %+v", syn)
	}
	if !spotNear(sale.Pnl, 0) {
		t.Fatalf("unknown basis must realize nothing, got pnl %v", sale.Pnl)
	}
}

func TestSpotBaseFeeReducesInventory(t *testing.T) {
	buy := spotTestLeg("BTC", "buy", 1, 100, 1)
	buy.FeeUSD, buy.FeeBase = 0, 0.001
	holdings := []helpers.SpotHolding{{Asset: "BTC", Qty: 0.999, AvgPx: 100, PriceUSD: 110}}

	_, open := buildSpotTest([]helpers.SpotLeg{buy}, holdings, spotTestStart, nil, map[string]float64{"BTC": 0.00001}, nil)
	if len(open) != 1 || !spotNear(open[0].Amount, 0.999) {
		t.Fatalf("want open 0.999, got %+v", open)
	}
	if len(open[0].Orders) != 1 || open[0].Orders[0].Status == helpers.OrderStatusSynthetic {
		t.Fatalf("the fee must not leave an inventory gap or a synthetic, got %+v", open[0].Orders)
	}
	if !spotNear(open[0].EntryPrice, 100) || !spotNear(open[0].Orders[0].AmountFilled, 1) || !spotNear(open[0].Commission, 0.1) {
		t.Fatalf("entry %v amount %v commission %v, want the fill price, the full fill and the fee once", open[0].EntryPrice, open[0].Orders[0].AmountFilled, open[0].Commission)
	}

	sell := spotTestLeg("BTC", "sell", 0.999, 120, 2)
	closed, open := buildSpotTest([]helpers.SpotLeg{buy, sell}, nil, spotTestStart, nil, map[string]float64{"BTC": 0.00001}, nil)
	if len(open) != 0 || len(closed) != 1 || len(closed[0].Orders) != 2 {
		t.Fatalf("selling the net quantity must close the lot without leftovers, got %d closed %d open", len(closed), len(open))
	}
	sale := closed[0]
	if !spotNear(sale.EntryPrice, 100) || !spotNear(sale.Pnl, 19.98) || !spotNear(sale.Commission, 0.11) || !spotNear(sale.NetPnl, 19.87) {
		t.Fatalf("gross pnl must ignore the fee and net must take it once, got entry %v pnl %v commission %v net %v", sale.EntryPrice, sale.Pnl, sale.Commission, sale.NetPnl)
	}
}

func TestSpotTodayNegativeAdjustmentIsCappedAtNow(t *testing.T) {
	now := spotTestStart.AddDate(0, 0, 10).Add(9 * time.Hour)
	today := helpers.SpotDayStart(now.UnixMilli())
	start := time.UnixMilli(today).UTC().AddDate(0, 0, -1)
	buy := helpers.SpotLeg{Asset: "BTC", Quote: "USDT", InstId: "BTC-USDT", OrdId: "b", OrdType: "market", Side: "buy", Qty: 0.1, PxUSD: 100, Ts: start.UnixMilli()}
	adjustments := []helpers.SpotAdjustment{{Asset: "BTC", Qty: -0.1, Ts: today}}

	closed, _ := BuildSpotPositions([]helpers.SpotLeg{buy}, nil, start, now, nil, map[string]float64{"BTC": 0.00001}, adjustments)
	if len(closed) != 1 {
		t.Fatalf("want one closed position, got %d", len(closed))
	}
	if !closed[0].ClosedAt.Equal(now) {
		t.Fatalf("closed at %v, want now %v", closed[0].ClosedAt, now)
	}

	closed, _ = BuildSpotPositions([]helpers.SpotLeg{buy}, nil, start, now.AddDate(0, 0, 1), nil, map[string]float64{"BTC": 0.00001}, adjustments)
	if want := time.UnixMilli(today + helpers.SpotDayMs - 1).UTC(); len(closed) != 1 || !closed[0].ClosedAt.Equal(want) {
		t.Fatalf("a past day closes at its end %v, got %+v", want, closed)
	}
}

func TestSpotAdjustmentBeforeTheWindowIsPlacedAtTheWindowStart(t *testing.T) {
	start := spotTestStart.Add(15 * time.Hour)
	adjustments := []helpers.SpotAdjustment{{Asset: "BTC", Qty: 0.1, Ts: spotTestDay(0)}}
	sell := spotTestLeg("BTC", "sell", 0.1, 110, 1)

	closed, open := buildSpotTest([]helpers.SpotLeg{sell}, nil, start, spotTestPrices(map[int]float64{0: 100}), map[string]float64{"BTC": 0.00001}, adjustments)
	if len(open) != 0 || len(closed) != 1 {
		t.Fatalf("want one closed position, got %d closed %d open", len(closed), len(open))
	}
	sale := closed[0]
	buy := sale.Orders[0]
	if buy.Status != helpers.OrderStatusAdjustment || !buy.Trade.DoneAt.Equal(start) || !sale.CreatedAt.Equal(start) {
		t.Fatalf("the adjustment buy must not precede the window start %v, got %+v", start, buy)
	}
	if spotOpeningSynthetics(sale.Orders) != 0 || !spotNear(sale.Pnl, 1) {
		t.Fatalf("want no opening synthetic and pnl 1, got %+v", sale)
	}
}

func TestSpotOpenLotKeepsItsOrders(t *testing.T) {
	legs := []helpers.SpotLeg{
		spotTestLeg("BTC", "buy", 1, 100, 1),
		spotTestLeg("BTC", "sell", 1, 110, 2),
		spotTestLeg("BTC", "buy", 2, 200, 3),
		spotTestLeg("BTC", "sell", 0.5, 210, 4),
	}
	holdings := []helpers.SpotHolding{{Asset: "BTC", Qty: 1.5, AvgPx: 200, PriceUSD: 220}}

	closed, open := buildSpotTest(legs, holdings, spotTestStart, nil, map[string]float64{"BTC": 0.00001}, nil)
	if len(closed) != 0 || len(open) != 1 {
		t.Fatalf("want one open position, got %d closed %d open", len(closed), len(open))
	}
	pos := open[0]
	if len(pos.Orders) != 4 || spotOpeningSynthetics(pos.Orders) != 0 {
		t.Fatalf("the live lot must keep its own executions without a synthetic, got %+v", pos.Orders)
	}
	if !spotNear(pos.Amount, 1.5) || !spotNear(pos.EntryPrice, 200) || !pos.OpenTime.Equal(spotTestStart.AddDate(0, 0, 1)) {
		t.Fatalf("unexpected open position %+v", pos)
	}
}

func TestSpotSyntheticsAtTheSameInstantGetDistinctKeys(t *testing.T) {
	legs := []helpers.SpotLeg{
		spotTestLeg("BTC", "sell", 1, 100, 1),
		spotTestLeg("BTC", "sell", 1, 100, 1),
		spotTestLeg("BTC", "buy", 2, 100, 1),
	}
	legs[1].OrdId = "second"

	closed, _ := buildSpotTest(legs, nil, spotTestStart, nil, map[string]float64{"BTC": 0.00001}, nil)
	if len(closed) != 1 {
		t.Fatalf("want one closed position, got %d", len(closed))
	}
	seen := map[string]bool{}
	for _, order := range closed[0].Orders {
		if order.Status != helpers.OrderStatusSynthetic {
			continue
		}
		if order.ExchangeOrderID == "" || seen[order.ExchangeOrderID] {
			t.Fatalf("synthetic keys must be present and unique, got %+v", closed[0].Orders)
		}
		seen[order.ExchangeOrderID] = true
	}
	if len(seen) != 2 {
		t.Fatalf("want two synthetics, got %d", len(seen))
	}
}

func TestSpotMergedMarketIsTheLatestLots(t *testing.T) {
	merged := mergeSpotLots([]spotLot{
		{start: spotTestStart.AddDate(0, 0, 3), quote: "USDT", instId: "BTC-USDT"},
		{start: spotTestStart.AddDate(0, 0, 1), quote: "USDC", instId: "BTC-USDC"},
	})
	if merged.quote != "USDT" || merged.instId != "BTC-USDT" || !merged.start.Equal(spotTestStart.AddDate(0, 0, 1)) {
		t.Fatalf("want the market of the latest lot, got %s %s from %v", merged.quote, merged.instId, merged.start)
	}
}
