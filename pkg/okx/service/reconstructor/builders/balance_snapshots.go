package builders

import (
	"sort"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
)

func BuildBalanceSnapshotsFromBills(
	currentBalance float64,
	bills []models.Bill,
) []domain.UserBalanceSnapshot {
	// Several bills can share a millisecond; the balance after the last of
	// them (the highest bill id) is the one to keep, whatever order the
	// archive returned them in.
	ordered := make([]models.Bill, len(bills))
	copy(ordered, bills)
	sort.SliceStable(ordered, func(i, j int) bool {
		ti, tj := helpers.MustInt64(ordered[i].Ts), helpers.MustInt64(ordered[j].Ts)
		if ti != tj {
			return ti < tj
		}
		return helpers.MustInt64(ordered[i].BillId) < helpers.MustInt64(ordered[j].BillId)
	})

	snapshots := make([]domain.UserBalanceSnapshot, 0, len(ordered)+1)

	for _, b := range ordered {
		if !strings.Contains(strings.ToUpper(b.Ccy), "USD") {
			continue
		}
		bal := helpers.MustFloat(b.Bal)
		ts := helpers.TimeFromMs(b.Ts)
		snapshots = append(snapshots, domain.UserBalanceSnapshot{
			CreatedAt: ts,
			Balance:   helpers.Round8(bal),
		})
	}

	snapshots = append(snapshots, domain.UserBalanceSnapshot{
		CreatedAt: time.Now().UTC(),
		Balance:   helpers.Round8(currentBalance),
	})

	sort.SliceStable(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})

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
