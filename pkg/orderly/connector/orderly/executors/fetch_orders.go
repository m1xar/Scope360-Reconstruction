package executors

import (
	"strconv"

	orderly "github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly/models"
)

const ordersPageSize = 500

func FetchFilledOrders(
	client *orderly.Client,
	symbol string,
	startTime, endTime int64,
) ([]models.OrderlyOrder, error) {
	params := map[string]string{
		"status": "INCOMPLETE",
	}

	if symbol != "" {
		params["symbol"] = symbol
	}
	if startTime > 0 {
		params["start_t"] = strconv.FormatInt(startTime, 10)
	}
	if endTime > 0 {
		params["end_t"] = strconv.FormatInt(endTime, 10)
	}

	incomplete, err := fetchAllPaged[models.OrderlyOrder](client, "/v1/orders", params, ordersPageSize)
	if err != nil {
		return nil, err
	}

	params["status"] = "COMPLETED"
	completed, err := fetchAllPaged[models.OrderlyOrder](client, "/v1/orders", params, ordersPageSize)
	if err != nil {
		return nil, err
	}

	all := append(incomplete, completed...)

	filtered := make([]models.OrderlyOrder, 0, len(all))
	for _, o := range all {
		if o.TotalExecutedQuantity > 0 {
			filtered = append(filtered, o)
		}
	}

	return filtered, nil
}
