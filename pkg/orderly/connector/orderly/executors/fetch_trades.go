package executors

import (
	"strconv"

	orderly "github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly/models"
)

const tradesPageSize = 500

func FetchAllTrades(
	client *orderly.Client,
	symbol string,
	startTime, endTime int64,
) ([]models.OrderlyTrade, error) {
	params := make(map[string]string)

	if symbol != "" {
		params["symbol"] = symbol
	}
	if startTime > 0 {
		params["start_t"] = strconv.FormatInt(startTime, 10)
	}
	if endTime > 0 {
		params["end_t"] = strconv.FormatInt(endTime, 10)
	}

	trades, err := fetchAllPaged[models.OrderlyTrade](client, "/v1/trades", params, tradesPageSize)
	if err != nil {
		return nil, err
	}

	filtered := make([]models.OrderlyTrade, 0, len(trades))
	for _, t := range trades {
		if t.ExecutedQuantity > 0 {
			filtered = append(filtered, t)
		}
	}

	return filtered, nil
}
