package executors

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
)

const candlesPath = "/api/v1/market/candles"

const candlesPageLimit = 1440

func standardToBlofin(interval string) string {
	for _, suffix := range []string{"h", "d", "w"} {
		if strings.HasSuffix(interval, suffix) {
			return interval[:len(interval)-1] + strings.ToUpper(suffix)
		}
	}
	return interval
}

func FetchCandles(client *resty.Client, baseURL, instID, bar string, startMs, endMs int64) ([]models.Candle, error) {
	bar = standardToBlofin(bar)
	var result []models.Candle
	after := fmt.Sprint(endMs)

	for {
		params := map[string]string{
			"instId": instID,
			"bar":    bar,
			"after":  after,
			"limit":  fmt.Sprintf("%d", candlesPageLimit),
		}
		if startMs > 0 {
			// before is exclusive: step back 1ms so a bar opening exactly at startMs is returned.
			params["before"] = fmt.Sprint(startMs - 1)
		}

		page, err := doWithRateLimit(func() ([]models.Candle, error) {
			return blofin.DoGet[[]models.Candle](client, baseURL, candlesPath, params)
		})
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

		next := page[len(page)-1].Ts
		if next == "" || next == after {
			break
		}
		if nextMs, convErr := strconv.ParseInt(next, 10, 64); convErr == nil && startMs > 0 && nextMs <= startMs {
			break
		}
		after = next

		if len(page) < candlesPageLimit {
			break
		}
	}

	return result, nil
}
