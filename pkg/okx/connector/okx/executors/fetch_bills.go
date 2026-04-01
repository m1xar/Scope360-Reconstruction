package executors

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
)

const billsArchivePath = "/api/v5/account/bills-archive"

const billsPageLimit = 100

const billsWindowSize = 90 * 24 * time.Hour

func FetchAllBillsByInstType(client *resty.Client, baseURL, instType string, startMs int64) ([]models.Bill, error) {
	var result []models.Bill
	now := time.Now().UnixMilli()

	if startMs <= 0 {
		startMs = now - billsWindowSize.Milliseconds()
	}

	windowEnd := now
	for windowEnd > startMs {
		windowBegin := windowEnd - billsWindowSize.Milliseconds()
		if windowBegin < startMs {
			windowBegin = startMs
		}

		after := ""
		for {
			params := map[string]string{
				"instType": instType,
				"limit":    fmt.Sprintf("%d", billsPageLimit),
				"begin":    fmt.Sprint(windowBegin),
				"end":      fmt.Sprint(windowEnd),
			}
			if after != "" {
				params["after"] = after
			}

			page, err := okx.DoGet[[]models.Bill](client, baseURL, billsArchivePath, params)
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
			if len(page) < billsPageLimit {
				break
			}
			after = page[len(page)-1].BillId
		}

		windowEnd = windowBegin
	}

	return result, nil
}

func FetchAllSwapAndFuturesBills(client *resty.Client, baseURL string, startMs int64) ([]models.Bill, error) {
	var swapBills, futuresBills []models.Bill
	var swapErr, futuresErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		swapBills, swapErr = FetchAllBillsByInstType(client, baseURL, "SWAP", startMs)
	}()
	go func() {
		defer wg.Done()
		futuresBills, futuresErr = FetchAllBillsByInstType(client, baseURL, "FUTURES", startMs)
	}()
	wg.Wait()

	return mergeInstTypeResults(swapBills, swapErr, futuresBills, futuresErr)
}
