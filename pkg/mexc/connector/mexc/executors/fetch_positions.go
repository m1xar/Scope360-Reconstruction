package executors

import (
	"fmt"

	"github.com/go-resty/resty/v2"
	mexc "github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
)

const historyPositionsPath = "/api/v1/private/position/list/history_positions"

const positionsPageSize = 100

// FetchAllHistoryPositions pages history_positions (newest first) and stops
// once a whole page was updated before sinceMs; 0 pages everything.
func FetchAllHistoryPositions(client *resty.Client, sinceMs int64) ([]models.HistoryPosition, error) {
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

		if len(data) < positionsPageSize || pageOlderThan(sinceMs, data, func(p models.HistoryPosition) int64 { return p.UpdateTime }) {
			break
		}
		page++
	}

	return result, nil
}

// pageOlderThan reports whether every row of a page is older than sinceMs.
// Pages come newest first, so nothing after such a page can be inside the
// window; checking the whole page tolerates slightly unordered rows.
func pageOlderThan[T any](sinceMs int64, page []T, timeOf func(T) int64) bool {
	if sinceMs <= 0 || len(page) == 0 {
		return false
	}
	for _, row := range page {
		if timeOf(row) >= sinceMs {
			return false
		}
	}
	return true
}

const openPositionsPath = "/api/v1/private/position/open_positions"

func FetchOpenPositions(client *resty.Client) ([]models.OpenPosition, error) {
	return mexc.DoGet[[]models.OpenPosition](client, openPositionsPath, nil)
}

const contractDetailPath = "/api/v1/contract/detail"

func FetchAllContractDetails(client *resty.Client) ([]models.ContractDetail, error) {
	return mexc.DoGet[[]models.ContractDetail](client, contractDetailPath, nil)
}

func FetchContractDetail(client *resty.Client, symbol string) (models.ContractDetail, error) {
	return mexc.DoGet[models.ContractDetail](client, contractDetailPath, map[string]string{
		"symbol": symbol,
	})
}
