package helpers

import (
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func AttachBalanceInit(positions *[]domain.Position, snapshots []domain.UserBalanceSnapshot) {
	if positions == nil || len(*positions) == 0 || len(snapshots) == 0 {
		return
	}

	sorted := make([]domain.UserBalanceSnapshot, len(snapshots))
	copy(sorted, snapshots)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
	})

	for i := range *positions {
		pos := &(*positions)[i]
		idx := sort.Search(len(sorted), func(k int) bool {
			return sorted[k].CreatedAt.After(pos.CreatedAt)
		}) - 1
		if idx >= 0 {
			pos.BalanceInit = Round8(sorted[idx].Balance)
		}
	}
}
