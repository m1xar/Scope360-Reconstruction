package executors

import (
	"fmt"
	"sort"
	"strconv"

	orderly "github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly/models"
)

const candlesMaxLimit = 1000

// supportedIntervals maps standard (Hyperliquid) interval format to Orderly API format.
var supportedIntervals = map[string]string{
	"1m": "1m", "5m": "5m", "15m": "15m", "30m": "30m",
	"1h": "1h", "4h": "4h", "12h": "12h",
	"1d": "1d", "1w": "1w", "1M": "1mon", "1y": "1y",
}

func FetchCandles(
	client *orderly.Client,
	symbol string,
	interval string,
	startTime, endTime int64,
) ([]models.OrderlyCandle, error) {
	orderlyInterval, ok := supportedIntervals[interval]
	if !ok {
		return nil, fmt.Errorf("orderly: unsupported candle interval %q, supported: 1m, 5m, 15m, 30m, 1h, 4h, 12h, 1d, 1w, 1M, 1y", interval)
	}

	var all []models.OrderlyCandle
	seen := make(map[int64]struct{})

	cursor := startTime

	for {
		params := map[string]string{
			"symbol": symbol,
			"type":   orderlyInterval,
			"limit":  strconv.Itoa(candlesMaxLimit),
		}

		var candles models.OrderlyResponse[models.CandleRows]
		if err := client.Get("/v1/kline", params, &candles); err != nil {
			return nil, fmt.Errorf("orderly fetch candles %s %s: %w", symbol, orderlyInterval, err)
		}

		if !candles.Success {
			return nil, fmt.Errorf("orderly fetch candles: API returned success=false")
		}

		newCount := 0
		for _, c := range candles.Data.Rows {
			if endTime > 0 && c.StartTimestamp > endTime {
				continue
			}
			if c.StartTimestamp < cursor {
				continue
			}
			if _, ok := seen[c.StartTimestamp]; ok {
				continue
			}
			seen[c.StartTimestamp] = struct{}{}
			all = append(all, c)
			newCount++
		}

		if newCount == 0 || len(candles.Data.Rows) < candlesMaxLimit {
			break
		}

		last := candles.Data.Rows[len(candles.Data.Rows)-1]
		if last.EndTimestamp <= cursor {
			break
		}
		cursor = last.EndTimestamp
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].StartTimestamp < all[j].StartTimestamp
	})

	return all, nil
}
