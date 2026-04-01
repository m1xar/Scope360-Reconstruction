package executors

import (
	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
)

func FetchHistoricalOrders(client *resty.Client, endpoint, user string) ([]models.HistoricalOrder, error) {
	var out []models.HistoricalOrder
	err := hyperliquid.DoRequest(client, endpoint, map[string]any{
		"type": "historicalOrders",
		"user": user,
	}, &out)
	return out, err
}
