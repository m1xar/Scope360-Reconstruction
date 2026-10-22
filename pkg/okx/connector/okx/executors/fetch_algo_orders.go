package executors

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
)

const (
	algoPendingPath   = "/api/v5/trade/orders-algo-pending"
	algoHistoryPath   = "/api/v5/trade/orders-algo-history"
	algoTPSLTypes     = "conditional,oco"
	algoHistoryStates = "effective,canceled"
	algoPageLimit     = 100
)

func fetchAlgoPages(client *resty.Client, baseURL, path string, params map[string]string, startMs int64) ([]models.AlgoOrder, error) {
	var result []models.AlgoOrder
	after := ""
	for {
		page := map[string]string{"limit": fmt.Sprintf("%d", algoPageLimit)}
		for k, v := range params {
			page[k] = v
		}
		if after != "" {
			page["after"] = after
		}

		rows, err := doWithRateLimit(func() ([]models.AlgoOrder, error) {
			return okx.DoGet[[]models.AlgoOrder](client, baseURL, path, page)
		})
		if err != nil {
			if after != "" && isHTTP5xx(err) {
				break
			}
			return nil, err
		}
		if len(rows) == 0 {
			break
		}

		reachedCutoff := false
		for _, row := range rows {
			if ctime, _ := strconv.ParseInt(row.CTime, 10, 64); startMs > 0 && ctime < startMs {
				reachedCutoff = true
				break
			}
			result = append(result, row)
		}
		if reachedCutoff || len(rows) < algoPageLimit {
			break
		}
		after = rows[len(rows)-1].AlgoId
	}
	return result, nil
}

func FetchAllAlgoOrders(client *resty.Client, baseURL string, startMs int64) ([]models.AlgoOrder, error) {
	var pending, history []models.AlgoOrder
	var pendingErr, historyErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		pending, pendingErr = fetchAlgoPages(client, baseURL, algoPendingPath, map[string]string{
			"ordType": algoTPSLTypes,
		}, startMs)
	}()
	go func() {
		defer wg.Done()
		history, historyErr = fetchAlgoPages(client, baseURL, algoHistoryPath, map[string]string{
			"ordType": algoTPSLTypes,
			"state":   algoHistoryStates,
		}, startMs)
	}()
	wg.Wait()

	return mergeInstTypeResults(pending, pendingErr, history, historyErr)
}
