package executors

import (
	"fmt"

	"github.com/go-resty/resty/v2"
	mexc "github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
)

const historyPositionsPath = "/api/v1/private/position/list/history_positions"

const positionsPageSize = 100

func FetchAllHistoryPositions(client *resty.Client) ([]models.HistoryPosition, error) {
	var result []models.HistoryPosition
	page := 1

	for {
		params := map[string]string{
			"page_num":  fmt.Sprint(page),
			"page_size": fmt.Sprint(positionsPageSize),
		}

		data, err := mexc.DoGet[[]models.HistoryPosition](client, historyPositionsPath, params)
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

		if len(data) < positionsPageSize {
			break
		}
		page++
	}

	return result, nil
}

const openPositionsPath = "/api/v1/private/position/open_positions"

func FetchOpenPositions(client *resty.Client) ([]models.OpenPosition, error) {
	return mexc.DoGet[[]models.OpenPosition](client, openPositionsPath, nil)
}
