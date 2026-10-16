package builders

import (
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

func BalanceEvents(closed []models.ClosedPosition, finance []models.FinanceOperation) []fx.BalanceEvent {
	events := make([]fx.BalanceEvent, 0, len(closed)+len(finance))
	for _, e := range closed {
		events = append(events, fx.BalanceEvent{Time: e.ClosedAt(), Delta: e.Net()})
	}
	for _, op := range finance {
		if delta, ok := financeDelta(op); ok {
			events = append(events, fx.BalanceEvent{Time: op.At(), Delta: delta})
		}
	}
	return events
}

func financeDelta(op models.FinanceOperation) (float64, bool) {
	kind := op.Kind()
	switch {
	case strings.Contains(kind, "COMMISSION"), strings.Contains(kind, "SWAP"), strings.Contains(kind, "CREDIT"):
		return 0, false
	case kind == models.FinanceWithdrawal:
		return -abs(op.Profit.Float()), true
	default:
		return op.Profit.Float(), true
	}
}

func SyntheticDeposit(current float64, events []fx.BalanceEvent, created time.Time) (fx.BalanceEvent, bool) {
	amount := fx.Round8(fx.StartingBalance(current, events))
	if amount <= 0 {
		return fx.BalanceEvent{}, false
	}
	at := created
	for _, e := range events {
		if at.IsZero() || !e.Time.After(at) {
			at = e.Time.Add(-time.Second)
		}
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return fx.BalanceEvent{Time: at.UTC(), Delta: amount}, true
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
