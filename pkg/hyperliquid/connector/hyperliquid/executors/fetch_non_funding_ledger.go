package executors

import (
	"sort"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
)

type nonFundingLedgerKey struct {
	Time int64
	Hash string
}

func FetchAllNonFundingLedgerUpdates(
	client *resty.Client,
	endpoint string,
	user string,
	startTime int64,
	endTime int64,
) ([]models.NonFundingLedgerUpdate, error) {
	var (
		result []models.NonFundingLedgerUpdate
		cur    = startTime
		seen   = make(map[nonFundingLedgerKey]struct{})
	)

	for {
		payload := map[string]any{
			"type":      "userNonFundingLedgerUpdates",
			"user":      user,
			"startTime": cur,
		}
		if endTime > 0 {
			payload["endTime"] = endTime
		}

		var page []models.NonFundingLedgerUpdate
		if err := hyperliquid.DoRequest(client, endpoint, payload, &page); err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		maxTime := cur
		added := 0
		for _, item := range page {
			key := nonFundingLedgerKey{Time: item.Time, Hash: item.Hash}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, item)
			added++
			if item.Time > maxTime {
				maxTime = item.Time
			}
		}
		if added == 0 || maxTime == cur {
			break
		}
		cur = maxTime
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Time == result[j].Time {
			return result[i].Hash < result[j].Hash
		}
		return result[i].Time < result[j].Time
	})
	return result, nil
}
