package executors

import (
	"strconv"

	orderly "github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly/models"
)

const algoOrdersPageSize = 100

func FetchAlgoOrders(
	client *orderly.Client,
	symbol string,
	startTime, endTime int64,
) ([]models.OrderlyAlgoOrder, error) {
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

	all, err := fetchAllPaged[models.OrderlyAlgoOrder](client, "/v1/algo/orders", params, algoOrdersPageSize)
	if err != nil {
		return nil, err
	}

	filtered := make([]models.OrderlyAlgoOrder, 0, len(all))
	for _, o := range all {
		switch o.AlgoType {
		case "TP_SL":
			filtered = append(filtered, o)
		}
	}

	return filtered, nil
}
