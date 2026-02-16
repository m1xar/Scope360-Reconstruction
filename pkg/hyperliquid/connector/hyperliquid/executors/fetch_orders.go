package executors

import (
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"net/http"
)

func FetchHistoricalOrders(client *http.Client, endpoint, user string) ([]models.HistoricalOrder, error) {
	var out []models.HistoricalOrder
	err := hyperliquid.DoRequest(client, endpoint, map[string]any{
		"type": "historicalOrders",
		"user": user,
	}, &out)
	return out, err
}
