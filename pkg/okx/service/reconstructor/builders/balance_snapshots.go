package builders

import (
	"sort"
	"strings"
	"time"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/service/reconstructor/helpers"
)

const (
	depositStateSuccess    = "2"
	withdrawalStateSuccess = "2"
)

type balanceEvent struct {
	ts    time.Time
	delta float64
}

func BuildBalanceSnapshots(
	currentBalance float64,
	closedPositions []models.ClosedPosition,
	deposits []models.Deposit,
	withdrawals []models.Withdrawal,
) []domain.UserBalanceSnapshot {
	var events []balanceEvent

	for _, cp := range closedPositions {
		delta := helpers.MustFloat(cp.RealizedPnl) +
			helpers.MustFloat(cp.Fee) +
			helpers.MustFloat(cp.FundingFee) +
			helpers.MustFloat(cp.LiqPenalty)
		events = append(events, balanceEvent{
			ts:    helpers.TimeFromMs(cp.UTime),
			delta: delta,
		})
	}

	for _, d := range deposits {
		if d.State != depositStateSuccess || !strings.Contains(strings.ToUpper(d.Ccy), "USD") {
			continue
		}
		events = append(events, balanceEvent{
			ts:    helpers.TimeFromMs(d.Ts),
			delta: helpers.MustFloat(d.Amt),
		})
	}

	for _, w := range withdrawals {
		if w.State != withdrawalStateSuccess || !strings.Contains(strings.ToUpper(w.Ccy), "USD") {
			continue
		}
		events = append(events, balanceEvent{
			ts:    helpers.TimeFromMs(w.Ts),
			delta: -(helpers.MustFloat(w.Amt) + helpers.MustFloat(w.Fee)),
		})
	}

	if len(events) == 0 {
		return []domain.UserBalanceSnapshot{{
			CreatedAt: time.Now().UTC(),
			Balance:   helpers.Round8(currentBalance),
		}}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].ts.After(events[j].ts)
	})

	snapshots := make([]domain.UserBalanceSnapshot, 0, len(events)+1)
	snapshots = append(snapshots, domain.UserBalanceSnapshot{
		CreatedAt: time.Now().UTC(),
		Balance:   helpers.Round8(currentBalance),
	})

	balance := currentBalance
	for _, ev := range events {
		balance -= ev.delta
		rounded := helpers.Round8(balance)

		lastIdx := len(snapshots) - 1
		if snapshots[lastIdx].CreatedAt.Equal(ev.ts) {
			snapshots[lastIdx].Balance = rounded
			continue
		}

		snapshots = append(snapshots, domain.UserBalanceSnapshot{
			CreatedAt: ev.ts,
			Balance:   rounded,
		})
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})

	return snapshots
}
