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
		FeeUSD:  -0.01,
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
