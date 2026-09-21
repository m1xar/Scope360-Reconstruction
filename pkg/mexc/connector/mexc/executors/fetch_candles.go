package executors

import (
	"fmt"

	"github.com/go-resty/resty/v2"
	mexc "github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
)

const candlesPath = "/api/v1/contract/kline/%s"

// standardToMexc maps standard (Hyperliquid) interval format to MEXC API format.
var standardToMexc = map[string]string{
	"1m": "Min1", "5m": "Min5", "15m": "Min15", "30m": "Min30",
	"1h": "Min60", "4h": "Hour4", "8h": "Hour8",
	"1d": "Day1", "1w": "Week1", "1M": "Month1",
}

// candlesPerRequest keeps each call under MEXC's 2000-bar response cap.
const candlesPerRequest = 1000

var intervalMs = map[string]int64{
	"1m": 60_000, "5m": 300_000, "15m": 900_000, "30m": 1_800_000,
	"1h": 3_600_000, "4h": 14_400_000, "8h": 28_800_000,
	"1d": 86_400_000, "1w": 604_800_000, "1M": 2_678_400_000,
}

func FetchCandles(client *resty.Client, symbol, interval string, startMs, endMs int64) ([]models.Candle, error) {
	mexcInterval, ok := standardToMexc[interval]
	if !ok {
		return nil, fmt.Errorf("mexc: unsupported candle interval %q", interval)
	}

	path := fmt.Sprintf(candlesPath, symbol)
	window := intervalMs[interval] * candlesPerRequest

	var result []models.Candle
	for from := startMs; from <= endMs; from += window {
		to := from + window - 1
		if to > endMs {
			to = endMs
		}
		params := map[string]string{
			"interval": mexcInterval,
			"start":    fmt.Sprint(from / 1000),
			"end":      fmt.Sprint(to / 1000),
		}

		columnar, err := mexc.DoGet[models.CandleColumnar](client, path, params)
		if err != nil {
			return nil, err
		}
		result = append(result, columnar.ToCandles()...)
	}
	return result, nil
}
