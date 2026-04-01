package executors

import (
	"sort"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
)

type fillKey struct {
	Time int64
	Tid  int64
}

func FetchAllFills(client *resty.Client, endpoint, user string) ([]models.RawFill, error) {
	var (
		startTime int64
		result    []models.RawFill
		seen      = make(map[fillKey]struct{})
	)

	for {
		var page []models.RawFill

		err := hyperliquid.DoRequest(client, endpoint, map[string]any{
			"type":            "userFillsByTime",
			"user":            user,
			"startTime":       startTime,
			"aggregateByTime": true,
		}, &page)
		if err != nil {
			return nil, err
		}

		if len(page) == 0 {
			break
		}

		maxTime := startTime
		newAdded := 0

		for _, f := range page {
			key := fillKey{f.Time, f.Tid}
			if _, ok := seen[key]; ok {
				continue
			}

			seen[key] = struct{}{}
			result = append(result, f)
			newAdded++

			if f.Time > maxTime {
				maxTime = f.Time
			}
		}

		if newAdded == 0 {
			break
		}

		startTime = maxTime
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Time == result[j].Time {
			return result[i].Tid < result[j].Tid
		}
		return result[i].Time < result[j].Time
	})

	return result, nil
}
