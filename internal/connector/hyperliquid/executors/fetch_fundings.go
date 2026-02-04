package executors

import (
	"hyperliquid-trade-reconstructor/internal/connector/hyperliquid"
	"hyperliquid-trade-reconstructor/internal/connector/hyperliquid/models"
	"net/http"
)

func FetchAllFunding(
	client *http.Client,
	endpoint string,
	user string,
	startTime int64,
) ([]models.FundingHistoryItem, error) {

	var (
		result []models.FundingHistoryItem
		cur    = startTime
	)

	err := hyperliquid.DoRequest(client, endpoint, map[string]any{
		"type":      "userFunding",
		"user":      user,
		"startTime": cur,
	}, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
