package builders

import (
	"sort"
	"time"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/domain"
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

	fixLeadingZeroAndBackfillWithSnapshots(fills, snapshots)
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
