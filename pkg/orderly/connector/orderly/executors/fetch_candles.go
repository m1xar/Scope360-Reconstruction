package executors

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	orderly "github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly/models"
)

func FetchCandles(
	client *orderly.Client,
	symbol string,
	interval string,
	startTime, endTime int64,
) ([]models.OrderlyCandle, error) {
	resolution, ok := supportedIntervals[interval]
	if !ok {
		return nil, fmt.Errorf("orderly: unsupported candle interval %q, supported: 1m, 5m, 15m, 30m, 1h, 4h, 12h, 1d, 1w, 1M, 1y", interval)
	}

	intervalMs, ok := intervalDurationMs(interval)
	if !ok {
		return nil, fmt.Errorf("orderly: unsupported candle interval %q", interval)
	}

	fromSec := int64(0)
	if startTime > 0 {
		fromSec = startTime / 1000
	}

	toSec := time.Now().Unix()
	if endTime > 0 {
		toSec = endTime / 1000
	}
	if toSec < fromSec {
		return nil, fmt.Errorf("orderly fetch candles: endTime must be >= startTime")
	}

	all := make([]models.OrderlyCandle, 0, candlesMaxLimit)
	seen := make(map[int64]struct{}, candlesMaxLimit)
	currentTo := toSec

	for {
		params := map[string]string{
			"symbol":     symbol,
			"resolution": resolution,
			"from":       strconv.FormatInt(fromSec, 10),
			"to":         strconv.FormatInt(currentTo, 10),
			"limit":      strconv.Itoa(candlesMaxLimit),
		}

		var resp klineHistoryResponse
		if err := client.Get(klineHistoryPath, params, &resp); err != nil {
			return nil, fmt.Errorf("orderly fetch kline history %s %s: %w", symbol, resolution, err)
		}

		if strings.EqualFold(resp.S, "no_data") || len(resp.T) == 0 {
			break
		}
		if !strings.EqualFold(resp.S, "ok") {
			return nil, fmt.Errorf("orderly fetch kline history: unexpected status %q", resp.S)
		}

		n := minLen(len(resp.T), len(resp.O), len(resp.C), len(resp.H), len(resp.L), len(resp.V), len(resp.A))
		if n == 0 {
			break
		}

		minTSec := maxInt64
		for i := 0; i < n; i++ {
			tSec := resp.T[i]
			if tSec < minTSec {
				minTSec = tSec
			}

			startMs := tSec * 1000
			if startTime > 0 && startMs < startTime {
				continue
			}
			if endTime > 0 && startMs > endTime {
				continue
			}
			if _, ok := seen[startMs]; ok {
				continue
			}

			seen[startMs] = struct{}{}
			all = append(all, models.OrderlyCandle{
				Open:           resp.O[i],
				Close:          resp.C[i],
				High:           resp.H[i],
				Low:            resp.L[i],
				Volume:         resp.V[i],
				Amount:         resp.A[i],
				Symbol:         symbol,
				Type:           resolution,
				StartTimestamp: startMs,
				EndTimestamp:   startMs + intervalMs,
			})
		}

		if n < candlesMaxLimit || minTSec <= fromSec {
			break
		}

		nextTo := minTSec - 1
		if nextTo >= currentTo {
			break
		}
		currentTo = nextTo
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].StartTimestamp < all[j].StartTimestamp
	})

	return all, nil
}
