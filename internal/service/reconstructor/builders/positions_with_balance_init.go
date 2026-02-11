package builders

import (
	"hyperliquid-trade-reconstructor/internal/domain"
	"sort"
	"time"
)

func AttachBalanceInitToPositions(
	positions *[]domain.Position,
	snapshots []domain.UserBalanceSnapshot,
) {
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
		idx := lastSnapshotBefore(sorted, pos.CreatedAt)

		if idx >= 0 {
			pos.BalanceInit = sorted[idx].Balance
			continue
		}
		if !pos.ClosedAt.IsZero() {
			idx = firstSnapshotAfter(sorted, *pos.ClosedAt)
			if idx >= 0 {
				pos.BalanceInit = sorted[idx].Balance - pos.NetPnl
			}
		}
	}
}

func lastSnapshotBefore(
	snapshots []domain.UserBalanceSnapshot,
	atTime time.Time,
) int {
	target := atTime.UnixNano()
	idx := sort.Search(len(snapshots), func(i int) bool {
		return snapshots[i].CreatedAt.UnixNano() > target
	})
	return idx - 1
}

func firstSnapshotAfter(
	snapshots []domain.UserBalanceSnapshot,
	at time.Time,
) int {
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
