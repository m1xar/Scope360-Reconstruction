package builders

import (
	"sort"
	"strings"
	"time"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/service/reconstructor/helpers"
)

func BuildBalanceSnapshotsFromBills(
	currentBalance float64,
	bills []models.Bill,
) []domain.UserBalanceSnapshot {
	snapshots := make([]domain.UserBalanceSnapshot, 0, len(bills)+1)

	for _, b := range bills {
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

	sort.Slice(snapshots, func(i, j int) bool {
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
