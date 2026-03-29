package workers

import (
	"sync"

	orderly "github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly/executors"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/service/reconstructor/helpers"
)

func StartCandleWorkers(
	client *orderly.Client,
	requests <-chan helpers.CandleRequest,
	workerCount int,
) {
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for req := range requests {
				candles, err := executors.FetchCandles(
					client,
					req.Symbol,
					req.Interval,
					req.StartMs,
					req.EndMs,
				)
				req.ReplyCh <- helpers.CandleResponse{Candles: candles, Err: err}
			}
		}()
	}
	go func() {
		wg.Wait()
	}()
}
