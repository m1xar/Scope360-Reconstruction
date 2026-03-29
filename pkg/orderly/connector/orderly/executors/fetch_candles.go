package executors

import (
	"fmt"
	"sort"
	"strconv"

	orderly "github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly/models"
)

const candlesMaxLimit = 1000

var supportedIntervals = map[string]bool{
	"1m": true, "5m": true, "15m": true, "30m": true,
	"1h": true, "4h": true, "12h": true,
	"1d": true, "1w": true, "1mon": true, "1y": true,
}

func FetchCandles(
	client *orderly.Client,
	symbol string,
	interval string,
	startTime, endTime int64,
) ([]models.OrderlyCandle, error) {
	if !supportedIntervals[interval] {
		return nil, fmt.Errorf("orderly: unsupported candle interval %q, supported: 1m, 5m, 15m, 30m, 1h, 4h, 12h, 1d, 1w, 1mon, 1y", interval)
	}

	var all []models.OrderlyCandle
	seen := make(map[int64]struct{})

	cursor := startTime

	for {
		params := map[string]string{
			"symbol": symbol,
			"type":   interval,
			"limit":  strconv.Itoa(candlesMaxLimit),
		}

		var candles models.OrderlyResponse[models.CandleRows]
		if err := client.Get("/v1/kline", params, &candles); err != nil {
			return nil, fmt.Errorf("orderly fetch candles %s %s: %w", symbol, interval, err)
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
