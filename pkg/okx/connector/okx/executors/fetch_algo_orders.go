package executors

import (
	"fmt"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
)

const algoOrdersHistoryPath = "/api/v5/trade/orders-algo-history"

const algoOrdersPageLimit = 100

func FetchAllAlgoOrders(client *resty.Client, baseURL, instType, ordType string) ([]models.AlgoOrder, error) {
	var result []models.AlgoOrder
	after := ""

	for {
		params := map[string]string{
			"instType": instType,
			"ordType":  ordType,
			"limit":    fmt.Sprintf("%d", algoOrdersPageLimit),
		}
		if after != "" {
			params["after"] = after
		}

		page, err := okx.DoGet[[]models.AlgoOrder](client, baseURL, algoOrdersHistoryPath, params)
		if err != nil {
			if after != "" && isHTTP5xx(err) {
				break
			}
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		result = append(result, page...)
		if len(page) < algoOrdersPageLimit {
			break
		}
		after = page[len(page)-1].AlgoId
	}

	return result, nil
}

func FetchAllSwapAndFuturesAlgoOrders(client *resty.Client, baseURL string) ([]models.AlgoOrder, error) {
	var swapOrders, futuresOrders []models.AlgoOrder
	var swapErr, futuresErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		swapOrders, swapErr = FetchAllAlgoOrders(client, baseURL, "SWAP", "conditional")
	}()
	go func() {
		defer wg.Done()
		futuresOrders, futuresErr = FetchAllAlgoOrders(client, baseURL, "FUTURES", "conditional")
	}()
	wg.Wait()

	return mergeInstTypeResults(swapOrders, swapErr, futuresOrders, futuresErr)
}
