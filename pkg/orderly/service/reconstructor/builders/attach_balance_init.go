package builders

import (
	"sort"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/service/reconstructor/helpers"
)

func AttachBalanceInitToPositions(
	positions *[]domain.Position,
	snapshots []domain.UserBalanceSnapshot,
) {
	if positions == nil || len(*positions) == 0 || len(snapshots) == 0 {
		return
	}

	snaps := make([]domain.UserBalanceSnapshot, len(snapshots))
	copy(snaps, snapshots)
	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].CreatedAt.Before(snaps[j].CreatedAt)
	})

	for i := range *positions {
		pos := &(*positions)[i]

		idx := lastSnapshotBefore(snaps, pos.CreatedAt)
		if idx >= 0 {
			pos.BalanceInit = helpers.Round8(snaps[idx].Balance)
			continue
		}

		if pos.ClosedAt != nil && !pos.ClosedAt.IsZero() {
			idx = firstSnapshotAfterNonZero(snaps, *pos.ClosedAt)
			if idx >= 0 {
				pos.BalanceInit = helpers.Round8(snaps[idx].Balance - pos.NetPnl)
			}
		}
	}
}

func lastSnapshotBefore(snapshots []domain.UserBalanceSnapshot, atTime time.Time) int {
	target := atTime.UnixNano()
	idx := sort.Search(len(snapshots), func(i int) bool {
		return snapshots[i].CreatedAt.UnixNano() > target
	})
	return idx - 1
}

func firstSnapshotAfterNonZero(snapshots []domain.UserBalanceSnapshot, at time.Time) int {
	start := sort.Search(len(snapshots), func(i int) bool {
		return snapshots[i].CreatedAt.After(at)
	})
	for i := start; i < len(snapshots); i++ {
		if snapshots[i].Balance != 0 {
			return i
		}
	}
	return -1
}
