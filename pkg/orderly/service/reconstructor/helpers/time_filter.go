package helpers

import (
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func FilterPositionsByClosedAt(positions []domain.Position, cutoff *time.Time) []domain.Position {
	if cutoff == nil {
		return positions
	}

	filtered := positions[:0]
	for _, pos := range positions {
		if pos.ClosedAt == nil {
			continue
		}
		if !pos.ClosedAt.Before(*cutoff) {
			filtered = append(filtered, pos)
		}
	}
	return filtered
}

func FilterBalanceSnapshotsByCreatedAt(
	snapshots []domain.UserBalanceSnapshot,
	cutoff *time.Time,
) []domain.UserBalanceSnapshot {
	if cutoff == nil {
		return snapshots
	}

	filtered := snapshots[:0]
	for _, snapshot := range snapshots {
		if !snapshot.CreatedAt.Before(*cutoff) {
			filtered = append(filtered, snapshot)
		}
	}
	return filtered
}

func FilterFundingsByCreatedAt(fundings []domain.UserFunding, cutoff *time.Time) []domain.UserFunding {
	if cutoff == nil {
		return fundings
	}

	filtered := fundings[:0]
	for _, funding := range fundings {
		if !funding.CreatedAt.Before(*cutoff) {
			filtered = append(filtered, funding)
		}
	}
	return filtered
}
