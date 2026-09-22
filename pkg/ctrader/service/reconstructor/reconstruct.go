package reconstructor

import (
	"context"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

// Detail selects how much of a closed position is filled in.
type Detail int

const (
	// PositionsOnly builds the positions without fetching candles.
	PositionsOnly Detail = iota
	// WithExcursions also fetches trendbars for MAE/MFE.
	WithExcursions
)

// ReconstructClosedPositions loads days of history (positions opened earlier
// are backfilled by id in LoadHistory), builds the closed positions and keeps
// those closed inside the window.
func ReconstructClosedPositions(ctx context.Context, c *connector.Client, days int, detail Detail) ([]domain.FXPosition, error) {
	deals, orders, symbols, session, err := helpers.LoadHistory(ctx, c, days)
	if err != nil {
		return nil, err
	}
	positions := builders.BuildFXPositions(deals, orders, symbols, session)
	if detail == WithExcursions {
		helpers.EnrichFXMAEMFE(ctx, c, positions, symbols)
	}
	if cutoff := helpers.CutoffFromDays(days); cutoff != nil {
		kept := positions[:0]
		for _, pos := range positions {
			if pos.ClosedAt != nil && !pos.ClosedAt.Before(*cutoff) {
				kept = append(kept, pos)
			}
		}
		positions = kept
	}
	return positions, nil
}
