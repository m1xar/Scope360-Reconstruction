package executors

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
)

const positionsHistoryPath = "/api/v5/account/positions-history"

const positionsPageLimit = 100

const positionsMaxAge = 90 * 24 * time.Hour

func FetchAllClosedPositionsByInstType(client *resty.Client, baseURL, instType string) ([]models.ClosedPosition, error) {
	var result []models.ClosedPosition
	after := ""
	cutoffMs := time.Now().Add(-positionsMaxAge).UnixMilli()

	for {
		params := map[string]string{
			"instType": instType,
			"limit":    fmt.Sprintf("%d", positionsPageLimit),
		}
		if after != "" {
			params["after"] = after
		}

		page, err := okx.DoGet[[]models.ClosedPosition](client, baseURL, positionsHistoryPath, params)
		if err != nil {
			if after != "" && isHTTP5xx(err) {
				break
			}
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		reachedCutoff := false
		for _, cp := range page {
			utime, _ := strconv.ParseInt(cp.UTime, 10, 64)
			if utime < cutoffMs {
				reachedCutoff = true
				break
			}
			result = append(result, cp)
		}
		if reachedCutoff {
			break
		}

		if len(page) < positionsPageLimit {
			break
		}
		after = page[len(page)-1].UTime
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
