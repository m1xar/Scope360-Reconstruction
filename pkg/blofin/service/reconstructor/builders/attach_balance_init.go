package builders

import (
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func AttachBalanceInit(positions *[]domain.Position, snapshots []domain.UserBalanceSnapshot) {
	if len(*positions) == 0 || len(snapshots) == 0 {
		return
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})

	for i := range *positions {
		pos := &(*positions)[i]
		for j := len(snapshots) - 1; j >= 0; j-- {
			if !snapshots[j].CreatedAt.After(pos.CreatedAt) {
				pos.BalanceInit = snapshots[j].Balance
				break
			}
		}
	}
}
