package executors

import (
	"hyperliquid-trade-reconstructor/internal/hyperliquid"
	"hyperliquid-trade-reconstructor/internal/hyperliquid/models"
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

	var page []models.FundingHistoryItem

	err := hyperliquid.DoRequest(client, endpoint, map[string]any{
		"type":      "userFunding",
		"user":      user,
		"startTime": cur,
	}, &page)
	if err != nil {
		return nil, err
	}

	result = append(result, page...)

	return result, nil
}
