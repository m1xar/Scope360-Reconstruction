package executors

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
)

const fillsHistoryPath = "/api/v5/trade/fills-history"

const fillsPageLimit = 100

const FillsRetention = 90 * 24 * time.Hour

func FetchFillsWindow(client *resty.Client, baseURL, instType string, beginMs, endMs int64) ([]models.Fill, error) {
	var result []models.Fill
	after := ""
	for {
		params := map[string]string{
			"instType": instType,
			"limit":    fmt.Sprintf("%d", fillsPageLimit),
			"begin":    fmt.Sprint(beginMs),
			"end":      fmt.Sprint(endMs),
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
	return result, nil
}

func FetchSwapAndFuturesFillsWindow(client *resty.Client, baseURL string, beginMs, endMs int64) ([]models.Fill, error) {
	var swapFills, futuresFills []models.Fill
	var swapErr, futuresErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		swapFills, swapErr = FetchFillsWindow(client, baseURL, "SWAP", beginMs, endMs)
	}()
	go func() {
		defer wg.Done()
		futuresFills, futuresErr = FetchFillsWindow(client, baseURL, "FUTURES", beginMs, endMs)
	}()
	wg.Wait()

	return mergeInstTypeResults(swapFills, swapErr, futuresFills, futuresErr)
}

func FetchAllFills(client *resty.Client, baseURL, instType string, startMs int64) ([]models.Fill, error) {
	var result []models.Fill
	now := time.Now().UnixMilli()

	windowEnd := now
	for windowEnd > startMs {
		windowBegin := windowEnd - WindowSize.Milliseconds()
		if windowBegin < startMs {
			windowBegin = startMs
		}
		page, err := FetchFillsWindow(client, baseURL, instType, windowBegin, windowEnd)
		if err != nil {
			return nil, err
		}
		result = append(result, page...)
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
