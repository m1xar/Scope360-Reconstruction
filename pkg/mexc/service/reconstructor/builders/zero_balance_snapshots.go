package builders

import (
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func BuildZeroBalanceSnapshots(positions []domain.Position) []domain.UserBalanceSnapshot {
	snapshots := make([]domain.UserBalanceSnapshot, 0, len(positions))
	for _, pos := range positions {
		if pos.ClosedAt == nil {
			continue
		}
		snapshots = append(snapshots, domain.UserBalanceSnapshot{
			CreatedAt: *pos.ClosedAt,
			Balance:   0,
		})
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})
	return snapshots
}
