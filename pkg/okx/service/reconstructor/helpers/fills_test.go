package helpers

import (
	"testing"

	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
)

// One isolated limit buy of 4000 contracts was split between two positions:
// 190 opened a position that a market sell closed 32s later, the remaining
// 3810 opened the next one. A concurrent cross order on the same instrument
// must not leak into either.
func cashcatFixture() ([]models.Fill, map[string]models.Order) {
	const inst = "CASHCAT-USDT-SWAP"
	fill := func(ord, side, sz, px, fee, pnl, ts string) models.Fill {
		return models.Fill{InstId: inst, OrdId: ord, Side: side, PosSide: "long", FillSz: sz, FillPx: px, Fee: fee, FillPnl: pnl, Ts: ts}
	}
	fills := []models.Fill{
		fill("limit", "buy", "1553", "0.1684", "-0.2", "0", "1789066700076"),
		fill("limit", "buy", "2257", "0.1684", "-0.3", "0", "1789066700075"),
		fill("cross", "buy", "5000", "0.169", "-0.6", "0", "1789066586564"),
		fill("close", "sell", "100", "0.1686", "-0.04", "0.2", "1789066547549"),
		fill("close", "sell", "90", "0.1685", "-0.03", "0.09", "1789066547549"),
		fill("limit", "buy", "190", "0.1684", "-0.02", "0", "1789066515446"),
	}
	parents := map[string]models.Order{
		"limit": {InstId: inst, OrdId: "limit", OrdType: "limit", Side: "buy", PosSide: "long", TdMode: "isolated", Sz: "4000", AccFillSz: "4000"},
		"close": {InstId: inst, OrdId: "close", OrdType: "market", Side: "sell", PosSide: "long", TdMode: "isolated", Sz: "190", AccFillSz: "190"},
		"cross": {InstId: inst, OrdId: "cross", OrdType: "limit", Side: "buy", PosSide: "long", TdMode: "cross", Sz: "5000", AccFillSz: "5000"},
	}
	return fills, parents
}

func TestOrdersFromFillsSplitsOrderBetweenPositions(t *testing.T) {
	fills, parents := cashcatFixture()
	instrument := models.Instrument{CtVal: "10"}

	closed := OrdersFromFills(fills, parents, PositionScope{
		InstId: "CASHCAT-USDT-SWAP", PosSide: "long", MgnMode: "isolated",
		FromMs: 1789066515446, ToMs: 1789066547549,
	}, instrument)
	if len(closed) != 2 {
		t.Fatalf("closed position: got %d orders, want 2: %+v", len(closed), closed)
	}
	byID := map[string]models.Order{}
	for _, o := range closed {
		byID[o.OrdId] = o
	}
	if got := MustFloat(byID["limit"].AccFillSz); got != 1900 {
		t.Errorf("closed: limit buy filled %v, want 1900", got)
	}
	if got := MustFloat(byID["limit"].Sz); got != 40000 {
		t.Errorf("closed: limit buy order size %v, want 40000", got)
	}
	if got := MustFloat(byID["close"].AccFillSz); got != 1900 {
		t.Errorf("closed: market sell filled %v, want 1900", got)
	}
	if got := MustFloat(byID["close"].Pnl); got < 0.2899 || got > 0.2901 {
		t.Errorf("closed: market sell pnl %v, want 0.29", got)
	}

	open := OrdersFromFills(fills, parents, PositionScope{
		InstId: "CASHCAT-USDT-SWAP", PosSide: "long", MgnMode: "isolated",
		FromMs: 1789066700075,
	}, instrument)
	if len(open) != 1 || open[0].OrdId != "limit" {
		t.Fatalf("open position: got %+v, want only the limit buy", open)
	}
	if got := MustFloat(open[0].AccFillSz); got != 38100 {
		t.Errorf("open: limit buy filled %v, want 38100", got)
	}
	if got := MustInt64(open[0].UTime); got != 1789066700076 {
		t.Errorf("open: last fill ts %v, want 1789066700076", got)
	}
}

func TestOrdersFromFillsKeepsMarginModesApart(t *testing.T) {
	fills, parents := cashcatFixture()

	cross := OrdersFromFills(fills, parents, PositionScope{
		InstId: "CASHCAT-USDT-SWAP", PosSide: "long", MgnMode: "cross",
		FromMs: 1789066562312,
	}, models.Instrument{CtVal: "10"})
	if len(cross) != 1 || cross[0].OrdId != "cross" {
		t.Fatalf("cross position: got %+v, want only the cross order", cross)
	}
	if got := MustFloat(cross[0].AccFillSz); got != 50000 {
		t.Errorf("cross: filled %v, want 50000", got)
	}
}

func TestOrdersFromFillsWithoutParentOrder(t *testing.T) {
	fills := []models.Fill{
		{InstId: "X-USDT-SWAP", OrdId: "a", Side: "sell", PosSide: "short", FillSz: "2", FillPx: "10", Ts: "100"},
		{InstId: "X-USDT-SWAP", OrdId: "a", Side: "sell", PosSide: "short", FillSz: "2", FillPx: "12", Ts: "105"},
		{InstId: "X-USDT-SWAP", OrdId: "b", Side: "buy", PosSide: "long", FillSz: "1", FillPx: "11", Ts: "103"},
	}
	orders := OrdersFromFills(fills, nil, PositionScope{
		InstId: "X-USDT-SWAP", PosSide: "short", MgnMode: "cross", FromMs: 100, ToMs: 105,
	}, models.Instrument{CtVal: "1"})
	if len(orders) != 1 {
		t.Fatalf("got %d orders, want 1", len(orders))
	}
	o := orders[0]
	if MustFloat(o.AccFillSz) != 4 || MustFloat(o.AvgPx) != 11 || o.Side != "sell" || MustInt64(o.UTime) != 105 {
		t.Errorf("synthesized order = %+v", o)
	}
}

func TestOrdersFromFillsToleratesStampSkew(t *testing.T) {
	// OKX stamped uTime 1ms before the ts of the closing fill; a buy right
	// after the close opens the next position and must stay out.
	fills := []models.Fill{
		{InstId: "ETH-USDT-SWAP", OrdId: "open", Side: "buy", PosSide: "long", FillSz: "500", FillPx: "2488.9", Ts: "1000"},
		{InstId: "ETH-USDT-SWAP", OrdId: "close", Side: "sell", PosSide: "long", FillSz: "226.97", FillPx: "2597.87", Ts: "5000"},
		{InstId: "ETH-USDT-SWAP", OrdId: "close", Side: "sell", PosSide: "long", FillSz: "273.03", FillPx: "2597.87", Ts: "5001"},
		{InstId: "ETH-USDT-SWAP", OrdId: "next", Side: "buy", PosSide: "long", FillSz: "10", FillPx: "2600", Ts: "5200"},
	}
	orders := OrdersFromFills(fills, nil, PositionScope{
		InstId: "ETH-USDT-SWAP", PosSide: "long", FromMs: 1000, ToMs: 5000,
	}, models.Instrument{CtVal: "0.1"})
	if len(orders) != 2 {
		t.Fatalf("got %d orders, want 2: %+v", len(orders), orders)
	}
	for _, o := range orders {
		if got := MustFloat(o.AccFillSz); got < 49.9999 || got > 50.0001 {
			t.Errorf("order %s filled %v, want 50", o.OrdId, got)
		}
	}
}
