package executors

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
)

const ordersArchivePath = "/api/v5/trade/orders-history-archive"

const ordersPageLimit = 100

const windowSize = 30 * 24 * time.Hour

func FetchAllOrders(client *resty.Client, baseURL, instType string, startMs int64) ([]models.Order, error) {
	var result []models.Order
	now := time.Now().UnixMilli()

	windowEnd := now
	for windowEnd > startMs {
		windowBegin := windowEnd - windowSize.Milliseconds()
		if windowBegin < startMs {
			windowBegin = startMs
		}

		after := ""
		for {
			params := map[string]string{
				"instType": instType,
				"limit":    fmt.Sprintf("%d", ordersPageLimit),
				"begin":    fmt.Sprint(windowBegin),
				"end":      fmt.Sprint(windowEnd),
			}
			if after != "" {
				params["after"] = after
			}

			page, err := okx.DoGet[[]models.Order](client, baseURL, ordersArchivePath, params)
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
			if len(page) < ordersPageLimit {
				break
			}
			after = page[len(page)-1].OrdId
		}

		windowEnd = windowBegin
	}

	return result, nil
}

func FetchAllSwapAndFuturesOrders(client *resty.Client, baseURL string, startMs int64) ([]models.Order, error) {
	var swapOrders, futuresOrders []models.Order
	var swapErr, futuresErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		swapOrders, swapErr = FetchAllOrders(client, baseURL, "SWAP", startMs)
	}()
	go func() {
		defer wg.Done()
		futuresOrders, futuresErr = FetchAllOrders(client, baseURL, "FUTURES", startMs)
	}()
	wg.Wait()

	return mergeInstTypeResults(swapOrders, swapErr, futuresOrders, futuresErr)
}
