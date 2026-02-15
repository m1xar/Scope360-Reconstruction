package builders

import (
	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/service/reconstructor/helpers"
	"sort"
	"time"
)

func AttachBalanceInitToPositions(
	positions *[]domain.Position,
	snapshots *[]domain.UserBalanceSnapshot,
) {
	if positions == nil || len(*positions) == 0 || snapshots == nil || len(*snapshots) == 0 {
		return
	}
	sort.Slice(*snapshots, func(i, j int) bool {
		return (*snapshots)[i].CreatedAt.Before((*snapshots)[j].CreatedAt)
	})

	cutoff, applied := fixLeadingZeroAndBackfillWithSnapshots(positions, snapshots)

	for i := range *positions {
		pos := &(*positions)[i]

		if applied && pos.CreatedAt.Before(cutoff) {
			continue
		}

		idx := lastSnapshotBefore(*snapshots, pos.CreatedAt)
		if idx >= 0 {
			pos.BalanceInit = helpers.Round8((*snapshots)[idx].Balance)
			continue
		}

		if !pos.ClosedAt.IsZero() {
			idx = firstSnapshotAfter(*snapshots, *pos.ClosedAt)
			if idx >= 0 {
				pos.BalanceInit = helpers.Round8((*snapshots)[idx].Balance - pos.NetPnl)
			}
		}
	}
}

func fixLeadingZeroAndBackfillWithSnapshots(
	positions *[]domain.Position,
	snapshots *[]domain.UserBalanceSnapshot,
) (cutoff time.Time, applied bool) {
	if positions == nil || len(*positions) == 0 || snapshots == nil || len(*snapshots) == 0 {
		return time.Time{}, false
	}

	snaps := *snapshots

	firstNormalIdx := -1
	for i := 0; i < len(snaps); i++ {
		if snaps[i].Balance != 0 {
			firstNormalIdx = i
			break
		}
	}
	if firstNormalIdx <= 0 {
		return time.Time{}, false
	}

	leadingZeroCount := firstNormalIdx

	zeroSnapTime := snaps[0].CreatedAt
	cutoff = snaps[firstNormalIdx].CreatedAt

	hasBeforeZero := false
	for i := range *positions {
		if (*positions)[i].CreatedAt.Before(zeroSnapTime) {
			hasBeforeZero = true
			break
		}
	}
	if !hasBeforeZero {
		return time.Time{}, false
	}

	snaps = append(snaps[:0], snaps[leadingZeroCount:]...)
	*snapshots = snaps

	earlyIdxs := make([]int, 0, len(*positions))
	for i := range *positions {
		if (*positions)[i].CreatedAt.Before(cutoff) {
			earlyIdxs = append(earlyIdxs, i)
		}
	}
	if len(earlyIdxs) == 0 {
		return cutoff, true
	}

	sort.Slice(earlyIdxs, func(a, b int) bool {
		return (*positions)[earlyIdxs[a]].CreatedAt.Before((*positions)[earlyIdxs[b]].CreatedAt)
	})

	snapBalance := balanceAtOrAfterNonZero(*snapshots, cutoff)
	if snapBalance == nil {
		return cutoff, true
	}
	currentBalance := *snapBalance

	synth := make([]domain.UserBalanceSnapshot, 0, len(earlyIdxs))

	for j := len(earlyIdxs) - 1; j >= 0; j-- {
		p := &(*positions)[earlyIdxs[j]]

		p.BalanceInit = helpers.Round8(currentBalance - p.NetPnl)
		currentBalance = p.BalanceInit

		synth = append(synth, domain.UserBalanceSnapshot{
			CreatedAt: p.CreatedAt,
			Balance:   p.BalanceInit,
		})
	}

	*snapshots = append(*snapshots, synth...)
	sort.Slice(*snapshots, func(i, j int) bool {
		return (*snapshots)[i].CreatedAt.Before((*snapshots)[j].CreatedAt)
	})

	*snapshots = dedupSnapshotsByCreatedAtKeepLast(*snapshots)

	return cutoff, true
}

func lastSnapshotBefore(snapshots []domain.UserBalanceSnapshot, atTime time.Time) int {
	target := atTime.UnixNano()
	idx := sort.Search(len(snapshots), func(i int) bool {
		return snapshots[i].CreatedAt.UnixNano() > target
	})
	return idx - 1
}

func firstSnapshotAfter(snapshots []domain.UserBalanceSnapshot, at time.Time) int {
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

func balanceAtOrAfterNonZero(snaps []domain.UserBalanceSnapshot, t time.Time) *float64 {
	start := sort.Search(len(snaps), func(i int) bool {
		return !snaps[i].CreatedAt.Before(t)
	})
	for i := start; i < len(snaps); i++ {
		if snaps[i].Balance != 0 {
			return &snaps[i].Balance
		}
	}
	return nil
}

func dedupSnapshotsByCreatedAtKeepLast(snaps []domain.UserBalanceSnapshot) []domain.UserBalanceSnapshot {
	if len(snaps) <= 1 {
		return snaps
	}

	out := make([]domain.UserBalanceSnapshot, 0, len(snaps))
	for i := 0; i < len(snaps); i++ {
		if len(out) == 0 {
			out = append(out, snaps[i])
			continue
		}
		last := &out[len(out)-1]
		if last.CreatedAt.Equal(snaps[i].CreatedAt) {
			*last = snaps[i]
			continue
		}
		out = append(out, snaps[i])
	}
	return out
}
