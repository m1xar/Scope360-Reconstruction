package executors

import (
	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
)

func FetchAllFunding(
	client *resty.Client,
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
