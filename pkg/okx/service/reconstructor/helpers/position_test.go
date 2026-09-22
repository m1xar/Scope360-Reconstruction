package helpers

import (
	"testing"

	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
)

// Open 100, close 50, add 50, close 100: the peak exposure is 100 but the
// opening orders add up to 150, and the position size must match them.
func TestBuildPositionAmountIsSumOfOpeningOrders(t *testing.T) {
	cp := models.ClosedPosition{
		InstId: "X-USDT-SWAP", Direction: "long", MgnMode: "cross", Lever: "5",
		OpenMaxPos: "10", OpenAvgPx: "100", CloseAvgPx: "110",
		Pnl: "150", RealizedPnl: "140", CTime: "1000", UTime: "5000",
	}
	instrument := models.Instrument{CtVal: "10"}
	orders := []models.Order{
		{OrdId: "1", Side: "buy", Sz: "100", AccFillSz: "100", AvgPx: "100", UTime: "1000"},
		{OrdId: "2", Side: "sell", Sz: "50", AccFillSz: "50", AvgPx: "110", UTime: "2000"},
		{OrdId: "3", Side: "buy", Sz: "50", AccFillSz: "50", AvgPx: "100", UTime: "3000"},
		{OrdId: "4", Side: "sell", Sz: "100", AccFillSz: "100", AvgPx: "110", UTime: "5000"},
	}

	pos, err := BuildPosition(cp, orders, instrument)
	if err != nil {
		t.Fatal(err)
	}
	if pos.Amount != 150 {
		t.Errorf("Amount = %v, want 150 (sum of opening orders)", pos.Amount)
	}

	var opened, closed float64
	for _, o := range pos.Orders {
		if o.Side == "BUY" {
			opened += o.AmountFilled
		} else {
			closed += o.AmountFilled
		}
	}
	if opened != pos.Amount || closed != pos.Amount {
		t.Errorf("orders opened %v / closed %v, position %v", opened, closed, pos.Amount)
	}
}

func TestBuildPositionAmountFallsBackToPeak(t *testing.T) {
	cp := models.ClosedPosition{
		InstId: "X-USDT-SWAP", Direction: "short", OpenMaxPos: "10",
		OpenAvgPx: "100", CloseAvgPx: "90", Pnl: "1000", RealizedPnl: "990",
		CTime: "1000", UTime: "5000",
	}
	pos, err := BuildPosition(cp, nil, models.Instrument{CtVal: "10"})
	if err != nil {
		t.Fatal(err)
	}
	if pos.Amount != 100 {
		t.Errorf("Amount = %v, want 100 (openMaxPos in base units)", pos.Amount)
	}
	for _, o := range pos.Orders {
		if o.AmountFilled != 100 {
			t.Errorf("synthetic %s order filled %v, want 100 in base units", o.Side, o.AmountFilled)
		}
	}
}
