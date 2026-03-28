package executors

import (
	"github.com/go-resty/resty/v2"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
)

const positionsHistoryPath = "/api/v5/account/positions-history"

func FetchAllClosedPositions(client *resty.Client, baseURL string) ([]models.ClosedPosition, error) {
	var result []models.ClosedPosition
	after := ""

	for {
		params := map[string]string{
			"instType": "SWAP",
		}
		if after != "" {
			params["after"] = after
		}

		page, err := okx.DoGet[[]models.ClosedPosition](client, baseURL, positionsHistoryPath, params)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		result = append(result, page...)
		after = page[len(page)-1].PosId
	}

	futuresAfter := ""
	for {
		params := map[string]string{
			"instType": "FUTURES",
		}
		if futuresAfter != "" {
			params["after"] = futuresAfter
		}

		page, err := okx.DoGet[[]models.ClosedPosition](client, baseURL, positionsHistoryPath, params)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		result = append(result, page...)
		futuresAfter = page[len(page)-1].PosId
	}

	return result, nil
}
