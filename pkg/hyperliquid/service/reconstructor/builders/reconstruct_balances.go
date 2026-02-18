package builders

import (
	"sort"
	"time"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
)

func ReconstructBalancesFromPositions(
	positions []domain.Position,
	snapshots *[]domain.UserBalanceSnapshot,
) {
	if len(positions) == 0 || snapshots == nil || len(*snapshots) == 0 {
		return
	}

	sort.Slice(*snapshots, func(i, j int) bool {
		return (*snapshots)[i].CreatedAt.Before((*snapshots)[j].CreatedAt)
	})

	fixLeadingZeroAndBackfillWithSnapshots(positions, snapshots)
}

func fixLeadingZeroAndBackfillWithSnapshots(
	positions []domain.Position,
	snapshots *[]domain.UserBalanceSnapshot,
) {
	if len(positions) == 0 || snapshots == nil || len(*snapshots) == 0 {
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
	for i := range positions {
		if positions[i].CreatedAt.Before(zeroSnapTime) {
			hasBeforeZero = true
			break
		}
	}
	if !hasBeforeZero {
		return
	}

	snaps = append(snaps[:0], snaps[leadingZeroCount:]...)
	*snapshots = snaps

	earlyIdxs := make([]int, 0, len(positions))
	for i := range positions {
		if positions[i].CreatedAt.Before(cutoff) {
			earlyIdxs = append(earlyIdxs, i)
		}
	}
	if len(earlyIdxs) == 0 {
		return
	}

	sort.Slice(earlyIdxs, func(a, b int) bool {
		return positions[earlyIdxs[a]].CreatedAt.Before(positions[earlyIdxs[b]].CreatedAt)
	})

	snapBalance := balanceAtOrAfterNonZero(*snapshots, cutoff)
	if snapBalance == nil {
		return
	}
	currentBalance := *snapBalance

	synth := make([]domain.UserBalanceSnapshot, 0, len(earlyIdxs))

	for j := len(earlyIdxs) - 1; j >= 0; j-- {
		p := positions[earlyIdxs[j]]
		balanceInit := helpers.Round8(currentBalance - p.NetPnl)
		currentBalance = balanceInit

		synth = append(synth, domain.UserBalanceSnapshot{
			CreatedAt: p.CreatedAt,
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
