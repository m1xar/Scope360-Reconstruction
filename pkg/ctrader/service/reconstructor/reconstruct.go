package reconstructor

import (
	"context"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
)

// ReconstructClosedPositions loads days of history (positions opened earlier
func ReconstructClosedPositions(ctx context.Context, c *connector.Client, days int) ([]domain.FXPosition, error) {
	d, err := Load(ctx, c, days, scope.Closed)
	if err != nil {
		return nil, err
	}
	return d.ClosedPositions(), nil
}
