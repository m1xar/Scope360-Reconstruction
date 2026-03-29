package executors

import (
	"strconv"

	orderly "github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly/models"
)

const fundingPageSize = 60

func FetchAllFunding(
	client *orderly.Client,
	symbol string,
	startTime, endTime int64,
) ([]models.OrderlyFunding, error) {
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

	return fetchAllPaged[models.OrderlyFunding](client, "/v1/funding_fee/history", params, fundingPageSize)
}
