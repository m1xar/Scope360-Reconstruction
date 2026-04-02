package executors

import (
	"fmt"

	"github.com/go-resty/resty/v2"
	mexc "github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
)

const candlesPath = "/api/v1/contract/kline/%s"

func FetchCandles(client *resty.Client, symbol, interval string, startMs, endMs int64) ([]models.Candle, error) {
	path := fmt.Sprintf(candlesPath, symbol)
	params := map[string]string{
		"interval": interval,
		"start":    fmt.Sprint(startMs / 1000),
		"end":      fmt.Sprint(endMs / 1000),
	}

	return mexc.DoGet[[]models.Candle](client, path, params)
}
