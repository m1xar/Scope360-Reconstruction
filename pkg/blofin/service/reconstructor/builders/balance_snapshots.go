package builders

import (
	"sort"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

type BalanceEvent struct {
	CreatedAt time.Time
	Delta     float64
}

func BuildSyntheticBalanceSnapshots(
	currentBalance float64,
	bills []models.Bill,
	positions []domain.Position,
) []domain.UserBalanceSnapshot {
	events := make([]BalanceEvent, 0, len(bills)+len(positions))
	for _, pos := range positions {
		if pos.ClosedAt == nil {
			continue
		}
		events = append(events, BalanceEvent{
			CreatedAt: *pos.ClosedAt,
			Delta:     pos.NetPnl,
		})
	}
	for _, bill := range bills {
		if delta, ok := futuresBillDelta(bill); ok && delta != 0 {
			events = append(events, BalanceEvent{
				CreatedAt: helpers.TimeFromMs(bill.TS),
				Delta:     delta,
			})
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt.After(events[j].CreatedAt)
	})

	snapshots := make([]domain.UserBalanceSnapshot, 0, len(events)+1)
	running := helpers.Round8(currentBalance)
	snapshots = append(snapshots, domain.UserBalanceSnapshot{
		CreatedAt: time.Now().UTC(),
		Balance:   running,
	})

	for _, event := range events {
		snapshots = append(snapshots, domain.UserBalanceSnapshot{
			CreatedAt: event.CreatedAt,
			Balance:   running,
		})
		running = helpers.Round8(running - event.Delta)
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})
	return dedupeSnapshots(snapshots)
}

func futuresBillDelta(bill models.Bill) (float64, bool) {
	if !helpers.IsStableCurrency(bill.Currency) {
		return 0, false
	}
	from := strings.ToLower(strings.TrimSpace(bill.FromAccount))
	to := strings.ToLower(strings.TrimSpace(bill.ToAccount))
	amount := helpers.MustFloat(bill.Amount)

	switch {
	case to == "futures" && from != "futures":
		return helpers.Round8(mathAbs(amount)), true
	case from == "futures" && to != "futures":
		return -helpers.Round8(mathAbs(amount)), true
	case to == "futures" && from == "":
		return helpers.Round8(amount), true
	case from == "futures" && to == "":
		return helpers.Round8(amount), true
	default:
		return 0, false
	}
}

func dedupeSnapshots(snapshots []domain.UserBalanceSnapshot) []domain.UserBalanceSnapshot {
	deduped := snapshots[:0]
	for i, s := range snapshots {
		if i > 0 && s.CreatedAt.Equal(snapshots[i-1].CreatedAt) {
			deduped[len(deduped)-1] = s
			continue
		}
		deduped = append(deduped, s)
	}
	return deduped
}

func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
