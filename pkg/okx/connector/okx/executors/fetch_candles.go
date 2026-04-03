package executors

import (
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
)

const historyCandlesPath = "/api/v5/market/history-candles"

const candlesPageLimit = 100

// standardToOkx converts standard (Hyperliquid) interval format to OKX bar format.
// OKX uses uppercase for hours/days/weeks: "1H","4H","12H","1D","3D","1W","1M".
func standardToOkx(interval string) string {
	for _, suffix := range []string{"h", "d", "w"} {
		if strings.HasSuffix(interval, suffix) {
			return interval[:len(interval)-1] + strings.ToUpper(suffix)
		}
	}
	return interval // "1m","5m","15m","30m","1M" stay the same
}

func FetchCandles(client *resty.Client, baseURL, instId, bar string, startMs, endMs int64) ([]models.Candle, error) {
	bar = standardToOkx(bar)
	var result []models.Candle
	after := fmt.Sprint(endMs)

	for {
		params := map[string]string{
			"instId": instId,
			"bar":    bar,
			"after":  after,
			"limit":  fmt.Sprintf("%d", candlesPageLimit),
		}
		if startMs > 0 {
			params["before"] = fmt.Sprint(startMs)
		}

		page, err := okx.DoGet[[]models.Candle](client, baseURL, historyCandlesPath, params)
		if err != nil {
			if len(result) > 0 && isHTTP5xx(err) {
				break
			}
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		result = append(result, page...)
		after = page[len(page)-1].Ts

		if len(page) < candlesPageLimit {
			break
		}
	}

	return result, nil
}
