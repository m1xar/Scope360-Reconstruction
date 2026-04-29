package workers

import (
	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/helpers"
)

func StartCandleWorkers(
	client *resty.Client,
	requests <-chan helpers.CandleRequest,
	workerCount int,
) {
	for i := 0; i < workerCount; i++ {
		go func() {
			for req := range requests {
				candles, err := executors.FetchCandles(client, req.TickType, req.Symbol, req.Interval, req.StartMs, req.EndMs)
				req.ReplyCh <- helpers.CandleResponse{Candles: candles, Err: err}
			}
		}()
	}
}
