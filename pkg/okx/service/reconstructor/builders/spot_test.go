package builders

import (
	"math"
	"testing"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
)

var spotTestStart = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

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

func TestSpotNegativeAdjustmentCapsOpenQuantityAtLiveBalance(t *testing.T) {
	legs := []helpers.SpotLeg{spotTestLeg("BTC", "buy", 0.05, 60000, 1)}
	holdings := []helpers.SpotHolding{{Asset: "BTC", Qty: 0.0185, PriceUSD: 65000}}
	adjustments := []helpers.SpotAdjustment{{Asset: "BTC", Qty: -0.0315, Ts: spotTestStart.AddDate(0, 0, 2).UnixMilli()}}

	closed, open := BuildSpotPositions(legs, holdings, spotTestStart, nil, map[string]float64{"BTC": 0.00001}, adjustments)
	if len(closed) != 0 || len(open) != 1 {
		t.Fatalf("want one open BTC holding, got %d closed %d open", len(closed), len(open))
	}
	if !spotNear(open[0].Amount, 0.0185) {
		t.Fatalf("open amount %v, want the live balance 0.0185", open[0].Amount)
	}
	for _, order := range open[0].Orders {
		if order.Status == helpers.OrderStatusSynthetic {
			t.Fatalf("explained inventory must not create a synthetic opening order")
		}
	}
}

func TestSpotPositiveAdjustmentKeepsTradedQuantity(t *testing.T) {
	legs := []helpers.SpotLeg{spotTestLeg("BTC", "buy", 0.05, 60000, 1)}
	holdings := []helpers.SpotHolding{{Asset: "BTC", Qty: 0.055, PriceUSD: 65000}}
	adjustments := []helpers.SpotAdjustment{{Asset: "BTC", Qty: 0.005, Ts: spotTestStart.AddDate(0, 0, 2).UnixMilli()}}

	_, open := BuildSpotPositions(legs, holdings, spotTestStart, nil, map[string]float64{"BTC": 0.00001}, adjustments)
	if len(open) != 1 || !spotNear(open[0].Amount, 0.05) {
		t.Fatalf("want open 0.05 BTC, got %+v", open)
	}
}

func TestSpotNegativeAdjustmentClosesLotWhenBalanceIsGone(t *testing.T) {
	legs := []helpers.SpotLeg{
		spotTestLeg("ETH", "buy", 0.003, 2500, 1),
		spotTestLeg("ETH", "sell", 0.0008, 2700, 3),
	}
	holdings := []helpers.SpotHolding{{Asset: "ETH", Qty: 0, PriceUSD: 2750}}
	adjustments := []helpers.SpotAdjustment{{Asset: "ETH", Qty: -0.0022, Ts: spotTestStart.AddDate(0, 0, 4).UnixMilli()}}

	closed, open := BuildSpotPositions(legs, holdings, spotTestStart, nil, map[string]float64{"ETH": 0.0001}, adjustments)
	if len(open) != 0 || len(closed) != 1 {
		t.Fatalf("want one closed ETH sale, got %d closed %d open", len(closed), len(open))
	}
	sale := closed[0]
	if !spotNear(sale.Amount, 0.0008) || !spotNear(sale.EntryPrice, 2500) || !spotNear(sale.ExitPrice, 2700) || !spotNear(sale.Pnl, 0.16) {
		t.Fatalf("unexpected sale %+v", sale)
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

	lots, _ := BuildSpotPositions(legs, holdings, spotTestStart, nil, minSizes, nil)
	if len(lots) != 2 {
		t.Fatalf("lot view: want 2 closed lots, got %d", len(lots))
	}

	closed, open := BuildSpotAssetPositions(legs, holdings, spotTestStart, nil, minSizes, nil)
	if len(open) != 0 || len(closed) != 1 {
		t.Fatalf("asset view: want one closed sale, got %d closed %d open", len(closed), len(open))
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
	var lotPnl, sellProfit float64
	for _, lot := range lots {
		lotPnl += lot.Pnl
	}
	for i, order := range sale.Orders {
		if order.PositionID != sale.ID {
			t.Fatalf("order %d belongs to another position", i)
		}
		if i > 0 && order.Trade.DoneAt.Before(sale.Orders[i-1].Trade.DoneAt) {
			t.Fatalf("executions are not in time order")
		}
		sellProfit += order.Trade.Profit
	}
	if !spotNear(lotPnl, sale.Pnl) || !spotNear(sellProfit, -10.02) {
		t.Fatalf("pnl must equal the lot sum: lots %v sale %v sells %v", lotPnl, sale.Pnl, sellProfit)
	}
}

func TestSpotAssetGroupsHeldAssetIntoOneOpenPosition(t *testing.T) {
	legs := append(spotTwoLots(), spotTestLeg("BTC", "sell", 1, 220, 4))
	holdings := []helpers.SpotHolding{{Asset: "BTC", Qty: 1, PriceUSD: 230}}
	minSizes := map[string]float64{"BTC": 0.00001}

	lots, lotOpen := BuildSpotPositions(legs, holdings, spotTestStart, nil, minSizes, nil)
	if len(lots) != 1 || len(lotOpen) != 1 || !spotNear(lotOpen[0].RealizedPnl, 20) {
		t.Fatalf("lot view: want 1 closed lot and the live lot, got %d closed %d open", len(lots), len(lotOpen))
	}

	closed, open := BuildSpotAssetPositions(legs, holdings, spotTestStart, nil, minSizes, nil)
	if len(closed) != 0 || len(open) != 1 {
		t.Fatalf("asset view: want one open position, got %d closed %d open", len(closed), len(open))
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
	closed, open := BuildSpotAssetPositions(nil, holdings, spotTestStart, nil, map[string]float64{"SOL": 0.001}, nil)
	if len(closed) != 0 || len(open) != 1 || !spotNear(open[0].Amount, 3) || !spotNear(open[0].EntryPrice, 150) {
		t.Fatalf("want one synthetic SOL holding, got %+v", open)
	}
	if len(open[0].Orders) != 1 || open[0].Orders[0].Status != helpers.OrderStatusSynthetic {
		t.Fatalf("want one synthetic opening order")
	}
}

func TestSpotAssetKeepsBuysWhoseUnitsLeftWithoutASell(t *testing.T) {
	legs := []helpers.SpotLeg{
		spotTestLeg("ETH", "buy", 1, 2000, 1),
		spotTestLeg("ETH", "sell", 1, 2100, 2),
		spotTestLeg("ETH", "buy", 0.5, 2200, 3),
	}
	holdings := []helpers.SpotHolding{{Asset: "ETH", Qty: 0, PriceUSD: 2300}}
	adjustments := []helpers.SpotAdjustment{{Asset: "ETH", Qty: -0.5, Ts: spotTestStart.AddDate(0, 0, 4).UnixMilli()}}
	minSizes := map[string]float64{"ETH": 0.0001}

	lots, _ := BuildSpotPositions(legs, holdings, spotTestStart, nil, minSizes, adjustments)
	if len(lots) != 1 || len(lots[0].Orders) != 2 {
		t.Fatalf("lot view: want one sale with 2 executions, got %+v", lots)
	}
	closed, open := BuildSpotAssetPositions(legs, holdings, spotTestStart, nil, minSizes, adjustments)
	if len(open) != 0 || len(closed) != 1 {
		t.Fatalf("asset view: want one closed sale, got %d closed %d open", len(closed), len(open))
	}
	sale := closed[0]
	if len(sale.Orders) != 3 || !spotNear(sale.Pnl, 100) || !spotNear(sale.Amount, 1) || !spotNear(sale.Commission, 0.03) {
		t.Fatalf("want all 3 executions with pnl 100 on 1 sold, got %d %v %v %v", len(sale.Orders), sale.Pnl, sale.Amount, sale.Commission)
	}
	if !sale.ClosedAt.Equal(spotTestStart.AddDate(0, 0, 2)) {
		t.Fatalf("closed at the last sell, got %v", sale.ClosedAt)
	}
}
