package executors

import (
	"context"
	"net/http"

	connector "github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
)

func FetchWorkingOrders(ctx context.Context, c *connector.Client) ([]models.Order, error) {
	path, err := c.AccountPath(ctx, "/orders")
	if err != nil {
		return nil, err
	}
	var list models.OrderList
	if err := c.Do(ctx, "orders", http.MethodGet, path, nil, nil, &list); err != nil {
		return nil, err
	}
	return list.Orders, nil
}
