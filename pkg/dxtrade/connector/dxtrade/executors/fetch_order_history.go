package executors

import (
	"context"
	"maps"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
)

const (
	historyPageLimit = 500
	maxPages         = 10000
	maxIDsPerQuery   = 50
)

func FetchOrderHistory(ctx context.Context, c *connector.Client, from *time.Time) ([]models.Order, error) {
	path, err := c.AccountPath(ctx, "/orders/history")
	if err != nil {
		return nil, err
	}
	base := url.Values{
		"limit":         {strconv.Itoa(historyPageLimit)},
		"include-links": {"true"},
	}
	if from != nil {
		base.Set("issued-from", models.FormatTime(*from))
	}
	byID := make(map[string]models.Order)
	var order []string
	cursor := ""
	for page := 0; page < maxPages; page++ {
		query := maps.Clone(base)
		if cursor != "" {
			query.Set("transaction-to", cursor)
		}
		var list models.OrderList
		if err := c.Do(ctx, "orders_history", http.MethodGet, path, query, nil, &list); err != nil {
			return nil, err
		}
		for _, o := range list.Orders {
			id := o.Key()
			prev, seen := byID[id]
			if !seen {
				order = append(order, id)
				byID[id] = o
			} else if o.Newer(prev) {
				byID[id] = o
			}
		}
		next := list.NextPageTransactionTime.Raw
		if next == "" || next == cursor {
			break
		}
		cursor = next
	}
	out := make([]models.Order, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].IssueTime.Before(out[j].IssueTime.Time) })
	return out, nil
}

func FetchOrdersByID(ctx context.Context, c *connector.Client, ids []string) ([]models.Order, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	path, err := c.AccountPath(ctx, "/orders/history")
	if err != nil {
		return nil, err
	}
	var out []models.Order
	for start := 0; start < len(ids); start += maxIDsPerQuery {
		end := min(start+maxIDsPerQuery, len(ids))
		query := url.Values{"with-order-id": {strings.Join(ids[start:end], ",")}}
		var list models.OrderList
		if err := c.Do(ctx, "orders_history_by_id", http.MethodGet, path, query, nil, &list); err != nil {
			return nil, err
		}
		out = append(out, list.Orders...)
	}
	return out, nil
}
