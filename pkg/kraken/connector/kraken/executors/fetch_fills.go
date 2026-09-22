package executors

import (
	"time"

	"github.com/go-resty/resty/v2"
	kraken "github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
)

const (
	fillsPath            = "/derivatives/api/v3/fills"
	FillsPageSize        = 100
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
