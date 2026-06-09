package executors

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/go-resty/resty/v2"
	blofin "github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
)

const maxCandleLimit = 1440

func FetchCandles(
	client *resty.Client,
	instID string,
	bar string,
	startMs int64,
	endMs int64,
) ([]models.Candle, error) {
	if endMs < startMs {
		return nil, fmt.Errorf("endMs must be >= startMs")
	}

	params := map[string]string{
		"instId": instID,
		"bar":    bar,
		"limit":  strconv.Itoa(maxCandleLimit),
		"after":  strconv.FormatInt(endMs+1, 10),
	}

	var all []models.Candle
	for {
		page, err := blofin.DoGet[[]models.Candle](
			client,
			"/api/v1/market/candles",
			params,
		)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		oldest := int64(0)
		for _, c := range page {
			ts := models.Int64(c.Ts)
			if oldest == 0 || ts < oldest {
				oldest = ts
			}
			if ts >= startMs && ts <= endMs {
				all = append(all, c)
			}
		}

		if len(page) < maxCandleLimit || oldest <= startMs {
			break
		}
		params["after"] = strconv.FormatInt(oldest, 10)
	}

	sort.Slice(all, func(i, j int) bool {
		return models.Int64(all[i].Ts) < models.Int64(all[j].Ts)
	})
	return all, nil
}
