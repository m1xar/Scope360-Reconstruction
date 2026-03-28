package executors

import (
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
)

const ordersArchivePath = "/api/v5/trade/orders-history-archive"

func FetchAllOrders(client *resty.Client, baseURL, instType string) ([]models.Order, error) {
	var result []models.Order
	after := ""

	for {
		params := map[string]string{
			"instType": instType,
			"limit":    "100",
		}
		if after != "" {
			params["after"] = after
		}

		page, err := okx.DoGet[[]models.Order](client, baseURL, ordersArchivePath, params)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		result = append(result, page...)
		after = page[len(page)-1].OrdId
	}

	return result, nil
}

func FetchAllSwapAndFuturesOrders(client *resty.Client, baseURL string) ([]models.Order, error) {
	var swapOrders, futuresOrders []models.Order
	var swapErr, futuresErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		swapOrders, swapErr = FetchAllOrders(client, baseURL, "SWAP")
	}()
	go func() {
		defer wg.Done()
		futuresOrders, futuresErr = FetchAllOrders(client, baseURL, "FUTURES")
	}()
	wg.Wait()

	return mergeInstTypeResults(swapOrders, swapErr, futuresOrders, futuresErr)
}
