package executors

import (
	"context"
	"net/http"

	connector "github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
)

func FetchPositions(ctx context.Context, c *connector.Client) ([]models.Position, error) {
	path, err := c.AccountPath(ctx, "/positions")
	if err != nil {
		return nil, err
	}
	var list models.PositionList
	if err := c.Do(ctx, "positions", http.MethodGet, path, nil, nil, &list); err != nil {
		return nil, err
	}
	return list.Positions, nil
}
