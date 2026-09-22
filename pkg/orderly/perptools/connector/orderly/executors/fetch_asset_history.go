package executors

import (
	"strconv"

	orderly "github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/models"
)

const assetHistoryPageSize = 100

// FetchAssetHistory returns deposits and withdrawals; startMs/endMs bound the
// range (0 = unbounded).
func FetchAssetHistory(client *orderly.Client, startMs, endMs int64) ([]models.OrderlyAssetHistory, error) {
	params := make(map[string]string)
	if startMs > 0 {
		params["start_t"] = strconv.FormatInt(startMs, 10)
	}
	if endMs > 0 {
		params["end_t"] = strconv.FormatInt(endMs, 10)
	}
	return fetchAllPaged[models.OrderlyAssetHistory](client, "/v1/asset/history", params, assetHistoryPageSize)
}
