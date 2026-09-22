package executors

import (
	"fmt"
	"sort"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
)

// FetchAllFunding pages userFunding from startTime to now. The endpoint caps
// a response, so pages advance by the newest time seen until nothing new
// comes back.
func FetchAllFunding(
	client *resty.Client,
	endpoint string,
	user string,
	startTime int64,
) ([]models.FundingHistoryItem, error) {
	var (
		result []models.FundingHistoryItem
		seen   = make(map[string]struct{})
		cursor = startTime
	)

	for {
		var page []models.FundingHistoryItem
		err := hyperliquid.DoRequest(client, endpoint, map[string]any{
			"type":      "userFunding",
			"user":      user,
			"startTime": cursor,
		}, &page)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		maxTime := cursor
		newAdded := 0
		for _, item := range page {
			key := fmt.Sprintf("%d|%s|%s|%s", item.Time, item.Hash, item.Delta.Coin, item.Delta.USDC)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, item)
			newAdded++
			if item.Time > maxTime {
				maxTime = item.Time
			}
		}
		if newAdded == 0 || maxTime <= cursor {
			break
		}
		cursor = maxTime
	}

	sort.SliceStable(result, func(i, j int) bool { return result[i].Time < result[j].Time })
	return result, nil
}
