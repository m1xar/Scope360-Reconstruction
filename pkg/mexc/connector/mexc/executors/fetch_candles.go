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

func FetchCandles(client *resty.Client, symbol, interval string, startMs, endMs int64) ([]models.Candle, error) {
	mexcInterval, ok := standardToMexc[interval]
	if !ok {
		return nil, fmt.Errorf("mexc: unsupported candle interval %q", interval)
	}

	path := fmt.Sprintf(candlesPath, symbol)
	params := map[string]string{
		"interval": mexcInterval,
		"start":    fmt.Sprint(startMs / 1000),
		"end":      fmt.Sprint(endMs / 1000),
	}

	return mexc.DoGet[[]models.Candle](client, path, params)
}
