package workers

import (
	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/helpers"
)

func StartCandleWorkers(
	client *resty.Client,
	requests <-chan helpers.CandleRequest,
	count int,
) {
	for i := 0; i < count; i++ {
		go func() {
			for req := range requests {
				candles, err := executors.FetchCandles(
					client,
					req.InstID,
					req.Bar,
					req.StartMs,
					req.EndMs,
				)
				req.ReplyCh <- helpers.CandleResponse{Candles: candles, Err: err}
			}
		}()
	}
}
