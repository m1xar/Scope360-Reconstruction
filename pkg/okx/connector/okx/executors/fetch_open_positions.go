package executors

import (
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
)

const positionsPath = "/api/v5/account/positions"

func FetchOpenPositions(client *resty.Client, baseURL string) ([]models.OpenPosition, error) {
	var swaps, futures []models.OpenPosition
	var swapErr, futuresErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		swaps, swapErr = okx.DoGet[[]models.OpenPosition](client, baseURL, positionsPath, map[string]string{"instType": "SWAP"})
	}()
	go func() {
		defer wg.Done()
		futures, futuresErr = okx.DoGet[[]models.OpenPosition](client, baseURL, positionsPath, map[string]string{"instType": "FUTURES"})
	}()
	wg.Wait()

	return mergeInstTypeResults(swaps, swapErr, futures, futuresErr)
}
