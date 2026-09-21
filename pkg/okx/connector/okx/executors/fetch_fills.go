package executors

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
)

// fills-history covers the last 3 months; begin/end filter by fill ts.
const fillsHistoryPath = "/api/v5/trade/fills-history"

const fillsPageLimit = 100

func FetchAllFills(client *resty.Client, baseURL, instType string, startMs int64) ([]models.Fill, error) {
	var result []models.Fill
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
				"limit":    fmt.Sprintf("%d", fillsPageLimit),
				"begin":    fmt.Sprint(windowBegin),
				"end":      fmt.Sprint(windowEnd),
			}
			if after != "" {
				params["after"] = after
			}

			page, err := doWithRateLimit(func() ([]models.Fill, error) {
				return okx.DoGet[[]models.Fill](client, baseURL, fillsHistoryPath, params)
			})
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
			if len(page) < fillsPageLimit {
				break
			}
			after = page[len(page)-1].BillId
		}

		windowEnd = windowBegin
	}

	return result, nil
}

func FetchAllSwapAndFuturesFills(client *resty.Client, baseURL string, startMs int64) ([]models.Fill, error) {
	var swapFills, futuresFills []models.Fill
	var swapErr, futuresErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		swapFills, swapErr = FetchAllFills(client, baseURL, "SWAP", startMs)
	}()
	go func() {
		defer wg.Done()
		futuresFills, futuresErr = FetchAllFills(client, baseURL, "FUTURES", startMs)
	}()
	wg.Wait()

	return mergeInstTypeResults(swapFills, swapErr, futuresFills, futuresErr)
}
