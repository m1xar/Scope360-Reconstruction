package executors

import (
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	mexc "github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
)

const historyOrdersPath = "/api/v1/private/order/list/history_orders"

const ordersPageSize = 100

const ordersWindowSize = 90 * 24 * time.Hour

func FetchAllHistoryOrders(client *resty.Client, startMs int64) ([]models.Order, error) {
	var result []models.Order
	now := time.Now().UnixMilli()

	if startMs <= 0 {
		startMs = now - ordersWindowSize.Milliseconds()
	}

	windowEnd := now
	for windowEnd > startMs {
		windowBegin := windowEnd - ordersWindowSize.Milliseconds()
		if windowBegin < startMs {
			windowBegin = startMs
		}

		page := 1
		for {
			params := map[string]string{
				"states":     "3",
				"start_time": fmt.Sprint(windowBegin),
				"end_time":   fmt.Sprint(windowEnd),
				"page_num":   fmt.Sprint(page),
				"page_size":  fmt.Sprint(ordersPageSize),
			}

			data, err := mexc.DoGet[[]models.Order](client, historyOrdersPath, params)
			if err != nil {
				if page > 1 && isHTTP5xx(err) {
					break
				}
				return nil, err
			}
			if len(data) == 0 {
				break
			}

			result = append(result, data...)

			if len(data) < ordersPageSize {
				break
			}
			page++
		}

		windowEnd = windowBegin
	}

	return result, nil
}
