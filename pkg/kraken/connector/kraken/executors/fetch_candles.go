package executors

import (
	"fmt"
	"net/url"
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	kraken "github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
)

const candlesPath = "/api/charts/v1/%s/%s/%s"
const candlesRateLimitedTries = 4

func FetchCandles(client *resty.Client, tickType, symbol, resolution string, startMs, endMs int64) ([]models.Candle, error) {
	path := fmt.Sprintf(
		candlesPath,
		url.PathEscape(tickType),
		url.PathEscape(symbol),
		url.PathEscape(resolution),
	)

	fromSec := startMs / 1000
	toSec := endMs / 1000
	seen := make(map[int64]struct{})
	candles := make([]models.Candle, 0)

	for toSec >= fromSec {
		params := map[string]string{
			"from": fmt.Sprint(fromSec),
			"to":   fmt.Sprint(toSec),
		}

		resp, err := kraken.DoGetWithRateLimitRetry[models.CandlesResponse](client, path, params, candlesRateLimitedTries)
		if err != nil {
			return nil, err
		}
		if len(resp.Candles) == 0 {
			break
		}

		oldestMs := resp.Candles[0].Time
		for _, candle := range resp.Candles {
			if candle.Time < oldestMs {
				oldestMs = candle.Time
			}
			if candle.Time < startMs || candle.Time > endMs {
				continue
			}
			if _, ok := seen[candle.Time]; ok {
				continue
			}
			seen[candle.Time] = struct{}{}
			candles = append(candles, candle)
		}

		if !resp.MoreCandles {
			break
		}

		nextToSec := oldestMs/1000 - 1
		if nextToSec >= toSec {
			break
		}
		toSec = nextToSec
		time.Sleep(250 * time.Millisecond)
	}

	sort.Slice(candles, func(i, j int) bool {
		return candles[i].Time < candles[j].Time
	})
	return candles, nil
}
