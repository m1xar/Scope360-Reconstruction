package executors

import (
	"fmt"

	orderly "github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/models"
)

func FetchSymbolsInfo(client *orderly.Client) ([]models.OrderlySymbolInfo, error) {
	var resp models.OrderlyResponse[models.OrderlySymbolsInfoResponse]
	if err := client.GetPublic("/v1/public/info", nil, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("orderly /v1/public/info: API returned success=false")
	}
	return resp.Data.Rows, nil
}
