package builders

import (
	"testing"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
)

func upd(old, cur, px float64, at int64) models.PositionUpdate {
	return models.PositionUpdate{OldPosition: models.FlexibleFloat(old), NewPosition: models.FlexibleFloat(cur), ExecutionPrice: models.FlexibleFloat(px), Timestamp: at, ExecutionUID: "x"}
}

func TestBuildOpenOrdersFromEventsAddsUpToLiveSize(t *testing.T) {
	// Flip from short 0.2 to long 0.1, add 0.3, liquidate 0.15 -> 0.25 long.
	events := []models.PositionUpdate{
		upd(-0.2, 0.1, 100, 1000),
		upd(0.1, 0.4, 101, 2000),
		upd(0.4, 0.25, 90, 3000),
	}
	if !IsOpeningEvent(events[0]) || IsOpeningEvent(events[1]) {
		t.Fatal("flip must be the opening event, an add must not")
	}
	orders := BuildOpenOrdersFromEvents(events, uuid.Nil)
	if len(orders) != 3 {
		t.Fatalf("orders = %d, want 3", len(orders))
	}
	var net float64
	for _, o := range orders {
		if o.Side == "BUY" {
			net += o.AmountFilled
		} else {
			net -= o.AmountFilled
		}
	}
	if net != 0.25 {
		t.Errorf("net of orders = %v, want 0.25", net)
	}
	if orders[0].AmountFilled != 0.1 || orders[2].Side != "SELL" {
		t.Errorf("orders = %+v", orders)
	}
}
