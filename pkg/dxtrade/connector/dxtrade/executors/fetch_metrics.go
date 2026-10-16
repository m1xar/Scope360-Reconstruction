package executors

import (
	"context"
	"net/http"
	"net/url"

	connector "github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
)

func FetchMetrics(ctx context.Context, c *connector.Client) (*models.AccountMetrics, error) {
	path, err := c.AccountPath(ctx, "/metrics")
	if err != nil {
		return nil, err
	}
	var list models.AccountMetricsList
	query := url.Values{"include-positions": {"true"}}
	if err := c.Do(ctx, "metrics", http.MethodGet, path, query, nil, &list); err != nil {
		return nil, err
	}
	if len(list.Metrics) == 0 {
		return &models.AccountMetrics{}, nil
	}
	return &list.Metrics[0], nil
}
