package workers

import (
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/executors"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/service/reconstructor/helpers"
)

func StartCandleWorkers(
	client *resty.Client,
	baseURL string,
	requests <-chan helpers.CandleRequest,
	workerCount int,
) {
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for req := range requests {
				candles, err := executors.FetchCandles(client, baseURL, req.InstId, req.Bar, req.StartMs, req.EndMs)
				req.ReplyCh <- helpers.CandleResponse{Candles: candles, Err: err}
			}
		}()
	}
	go func() {
		wg.Wait()
	}()
}
