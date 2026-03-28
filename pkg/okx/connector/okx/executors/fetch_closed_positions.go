package executors

import (
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
)

const positionsHistoryPath = "/api/v5/account/positions-history"

func FetchAllClosedPositionsByInstType(client *resty.Client, baseURL, instType string) ([]models.ClosedPosition, error) {
	var result []models.ClosedPosition
	after := ""

	for {
		params := map[string]string{
			"instType": instType,
			"limit":    "100",
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

	return result, nil
}

func FetchAllClosedPositions(client *resty.Client, baseURL string) ([]models.ClosedPosition, error) {
	var swapPositions, futuresPositions []models.ClosedPosition
	var swapErr, futuresErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		swapPositions, swapErr = FetchAllClosedPositionsByInstType(client, baseURL, "SWAP")
	}()
	go func() {
		defer wg.Done()
		futuresPositions, futuresErr = FetchAllClosedPositionsByInstType(client, baseURL, "FUTURES")
	}()
	wg.Wait()

	return mergeInstTypeResults(swapPositions, swapErr, futuresPositions, futuresErr)
}
