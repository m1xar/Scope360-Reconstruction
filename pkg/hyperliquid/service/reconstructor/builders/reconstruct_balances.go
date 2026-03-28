package builders

import (
	"sort"
	"time"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
)

func ReconstructBalancesFromRawFills(
	fills []models.RawFill,
	snapshots *[]domain.UserBalanceSnapshot,
) {
	if len(fills) == 0 || snapshots == nil || len(*snapshots) == 0 {
		return
	}

	sort.Slice(*snapshots, func(i, j int) bool {
		return (*snapshots)[i].CreatedAt.Before((*snapshots)[j].CreatedAt)
	})

	sort.Slice(fills, func(i, j int) bool {
		return fills[i].Time < fills[j].Time
	})

	fixLeadingZeroAndBackfillWithSnapshots(fills, snapshots)

	sort.Slice(*snapshots, func(i, j int) bool {
		return (*snapshots)[i].CreatedAt.Before((*snapshots)[j].CreatedAt)
	})

	ensureSnapshotsForFillGaps(fills, snapshots)
}

func fixLeadingZeroAndBackfillWithSnapshots(
	fills []models.RawFill,
	snapshots *[]domain.UserBalanceSnapshot,
) {
	if len(fills) == 0 || snapshots == nil || len(*snapshots) == 0 {
		return
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
		return
	}

	leadingZeroCount := firstNormalIdx

	zeroSnapTime := snaps[0].CreatedAt
	cutoff := snaps[firstNormalIdx].CreatedAt

	hasBeforeZero := false
	for i := range fills {
		if time.UnixMilli(fills[i].Time).Before(zeroSnapTime) {
			hasBeforeZero = true
			break
		}
	}
	if !hasBeforeZero {
		return
	}

	snaps = append(snaps[:0], snaps[leadingZeroCount:]...)
	*snapshots = snaps

	earlyIdxs := make([]int, 0, len(fills))
	for i := range fills {
		if time.UnixMilli(fills[i].Time).Before(cutoff) {
			earlyIdxs = append(earlyIdxs, i)
		}
	}
	if len(earlyIdxs) == 0 {
		return
	}

	sort.Slice(earlyIdxs, func(a, b int) bool {
		return fills[earlyIdxs[a]].Time < fills[earlyIdxs[b]].Time
	})

	snapBalance := balanceAtOrAfterNonZero(*snapshots, cutoff)
	if snapBalance == nil {
		return
	}
	currentBalance := *snapBalance

	synth := make([]domain.UserBalanceSnapshot, 0, len(earlyIdxs))

	for j := len(earlyIdxs) - 1; j >= 0; j-- {
		f := fills[earlyIdxs[j]]
		pnl := helpers.MustFloat(f.ClosedPnl)
		fee := helpers.MustFloat(f.Fee)
		balanceInit := helpers.Round8(currentBalance - pnl + fee)
		currentBalance = balanceInit

		synth = append(synth, domain.UserBalanceSnapshot{
			CreatedAt: time.UnixMilli(f.Time),
			Balance:   balanceInit,
		})
	}

	*snapshots = append(*snapshots, synth...)
	sort.Slice(*snapshots, func(i, j int) bool {
		return (*snapshots)[i].CreatedAt.Before((*snapshots)[j].CreatedAt)
	})

	*snapshots = dedupSnapshotsByCreatedAtKeepLast(*snapshots)
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

func ensureSnapshotsForFillGaps(
	fills []models.RawFill,
	snapshots *[]domain.UserBalanceSnapshot,
) {
	if len(fills) == 0 || snapshots == nil || len(*snapshots) == 0 {
		return
	}

	snaps := *snapshots
	sort.Slice(fills, func(i, j int) bool {
		return fills[i].Time < fills[j].Time
	})

	intervalTimes := make([]time.Time, 0, len(fills))
	intervalTimes = append(intervalTimes, time.UnixMilli(fills[0].Time))
	for i := 1; i < len(fills); i++ {
		if fills[i].Time != fills[i-1].Time {
			intervalTimes = append(intervalTimes, time.UnixMilli(fills[i].Time))
		}
	}

	synth := make([]domain.UserBalanceSnapshot, 0, len(intervalTimes))
	var prev *time.Time
	for _, end := range intervalTimes {
		if snapshotExistsInRange(snaps, prev, end) {
			prev = &end
			continue
		}

		anchorIdx := firstNonZeroSnapshotAfter(snaps, end)
		if anchorIdx < 0 {
			prev = &end
			continue
		}

		anchor := snaps[anchorIdx]
		balance, ok := backfillBalanceAtTime(fills, anchor, end)
		if !ok {
			prev = &end
			continue
		}

		synth = append(synth, domain.UserBalanceSnapshot{
			CreatedAt: end,
			Balance:   balance,
		})
		prev = &end
	}

	if len(synth) == 0 {
		return
	}

	*snapshots = append(*snapshots, synth...)
	sort.Slice(*snapshots, func(i, j int) bool {
		return (*snapshots)[i].CreatedAt.Before((*snapshots)[j].CreatedAt)
	})
	*snapshots = dedupSnapshotsByCreatedAtKeepLast(*snapshots)
}

func snapshotExistsInRange(
	snaps []domain.UserBalanceSnapshot,
	startExclusive *time.Time,
	endInclusive time.Time,
) bool {
	if len(snaps) == 0 {
		return false
	}

	if startExclusive == nil {
		idx := sort.Search(len(snaps), func(i int) bool {
			return snaps[i].CreatedAt.After(endInclusive)
		})
		return idx > 0
	}

	start := *startExclusive
	if !start.Before(endInclusive) {
		return false
	}
	idx := sort.Search(len(snaps), func(i int) bool {
		return snaps[i].CreatedAt.After(start)
	})
	if idx >= len(snaps) {
		return false
	}
	return !snaps[idx].CreatedAt.After(endInclusive)
}

func firstNonZeroSnapshotAfter(snaps []domain.UserBalanceSnapshot, at time.Time) int {
	start := sort.Search(len(snaps), func(i int) bool {
		return snaps[i].CreatedAt.After(at)
	})
	for i := start; i < len(snaps); i++ {
		if snaps[i].Balance != 0 {
			return i
		}
	}
	return -1
}

func backfillBalanceAtTime(
	fills []models.RawFill,
	anchor domain.UserBalanceSnapshot,
	target time.Time,
) (float64, bool) {
	anchorMs := anchor.CreatedAt.UnixMilli()
	targetMs := target.UnixMilli()
	if anchorMs <= targetMs {
		return 0, false
	}

	endIdx := sort.Search(len(fills), func(i int) bool {
		return fills[i].Time > anchorMs
	}) - 1
	if endIdx < 0 {
		return anchor.Balance, true
	}

	currentBalance := anchor.Balance
	for i := endIdx; i >= 0; i-- {
		t := fills[i].Time
		if t <= targetMs {
			break
		}
		pnl := helpers.MustFloat(fills[i].ClosedPnl)
		fee := helpers.MustFloat(fills[i].Fee)
		currentBalance = helpers.Round8(currentBalance - pnl + fee)
	}
	return currentBalance, true
}
