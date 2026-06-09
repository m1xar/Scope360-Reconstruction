package helpers

import (
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func FilterPositionsByDays(positions []domain.Position, days int) []domain.Position {
	cutoff := CutoffFromDays(days)
	if cutoff == nil {
		return positions
	}

	filtered := positions[:0]
	for _, pos := range positions {
		if pos.ClosedAt != nil && !pos.ClosedAt.Before(*cutoff) {
			filtered = append(filtered, pos)
		}
	}
	return filtered
}

func FilterSnapshotsByDays(snapshots []domain.UserBalanceSnapshot, days int) []domain.UserBalanceSnapshot {
	cutoff := CutoffFromDays(days)
	if cutoff == nil {
		return snapshots
	}

	filtered := snapshots[:0]
	for _, s := range snapshots {
		if !s.CreatedAt.Before(*cutoff) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func TimePtr(t time.Time) *time.Time {
	return &t
}
