package executors

import (
	"fmt"
	"net/url"
	"sort"

	"github.com/go-resty/resty/v2"
	kraken "github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
)

const candlesPath = "/api/charts/v1/%s/%s/%s"

func FetchCandles(client *resty.Client, tickType, symbol, resolution string, startMs, endMs int64) ([]models.Candle, error) {
	path := fmt.Sprintf(
		candlesPath,
		url.PathEscape(tickType),
		url.PathEscape(symbol),
		url.PathEscape(resolution),
	)

	params := map[string]string{
		"from": fmt.Sprint(startMs / 1000),
		"to":   fmt.Sprint(endMs / 1000),
	}

	resp, err := kraken.DoGet[models.CandlesResponse](client, path, params)
	if err != nil {
		return nil, err
	}

	candles := resp.Candles
	sort.Slice(candles, func(i, j int) bool {
		return candles[i].Time < candles[j].Time
	})
	return candles, nil
}
