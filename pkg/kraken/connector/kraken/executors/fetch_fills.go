package executors

import (
	"fmt"
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	kraken "github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
)

const (
	fillsPath            = "/derivatives/api/v3/fills"
	fillsPageSize        = 100
	fillRateLimitedTries = 4
	fillPaginatedPace    = 600 * time.Millisecond
)

func FetchFills(client *resty.Client, lastFillTime string) ([]models.Fill, error) {
	params := make(map[string]string)
	if lastFillTime != "" {
		time.Sleep(fillPaginatedPace)
		params["lastFillTime"] = lastFillTime
	}

	resp, err := kraken.DoGetWithRateLimitRetry[models.FillResponse](client, fillsPath, params, fillRateLimitedTries)
	if err != nil {
		return nil, err
	}
	return resp.Fills, nil
}

func FetchAllFills(client *resty.Client, days int) ([]models.Fill, error) {
	var cutoff *time.Time
	if days > 0 {
		t := time.Now().AddDate(0, 0, -days)
		cutoff = &t
	}

	var result []models.Fill
	seen := make(map[string]struct{})
	lastFillTime := ""

	for {
		page, err := FetchFills(client, lastFillTime)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		oldest := time.Time{}
		newAdded := 0
		for _, fill := range page {
			fillTime, err := ParseKrakenTime(fill.FillTime)
			if err != nil {
				continue
			}
			if cutoff != nil && fillTime.Before(*cutoff) {
				continue
			}

			key := fill.FillID
			if key == "" {
				key = fmt.Sprintf("%s|%s|%s|%.12f|%.12f", fill.Symbol, fill.OrderID, fill.FillTime, fill.Size.Float64(), fill.Price.Float64())
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, fill)
			newAdded++
		}

		for _, fill := range page {
			fillTime, err := ParseKrakenTime(fill.FillTime)
			if err != nil {
				continue
			}
			if oldest.IsZero() || fillTime.Before(oldest) {
				oldest = fillTime
			}
		}

		if oldest.IsZero() || newAdded == 0 {
			break
		}
		if cutoff != nil && oldest.Before(*cutoff) {
			break
		}
		if len(page) < fillsPageSize {
			break
		}
		lastFillTime = FormatKrakenTime(oldest)
	}

	sort.Slice(result, func(i, j int) bool {
		ti, _ := ParseKrakenTime(result[i].FillTime)
		tj, _ := ParseKrakenTime(result[j].FillTime)
		if ti.Equal(tj) {
			return result[i].FillID < result[j].FillID
		}
		return ti.Before(tj)
	})

	return result, nil
}
