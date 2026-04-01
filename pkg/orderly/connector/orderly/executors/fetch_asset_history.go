package executors

import (
	orderly "github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly/models"
)

const assetHistoryPageSize = 100

func FetchAssetHistory(client *orderly.Client) ([]models.OrderlyAssetHistory, error) {
	return fetchAllPaged[models.OrderlyAssetHistory](client, "/v1/asset/history", nil, assetHistoryPageSize)
}
