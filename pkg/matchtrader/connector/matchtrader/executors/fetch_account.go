package executors

import (
	"context"
	"net/http"

	connector "github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
)

func FetchBalance(ctx context.Context, c *connector.Client) (*models.Balance, error) {
	var b models.Balance
	if err := c.Do(ctx, http.MethodGet, "balance", nil, nil, &b); err != nil {
		return nil, err
	}
	return &b, nil
}
