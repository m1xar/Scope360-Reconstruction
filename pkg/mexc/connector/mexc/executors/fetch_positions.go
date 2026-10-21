package executors

import (
	"github.com/go-resty/resty/v2"
	mexc "github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
)

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
