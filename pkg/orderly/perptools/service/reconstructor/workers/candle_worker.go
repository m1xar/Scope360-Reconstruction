package workers

import (
	"sync"

	orderly "github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/models"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/candlespan"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/excursion"
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
				candles, err := fetchSpan(client, req)
				req.ReplyCh <- helpers.CandleResponse{Candles: candles, Err: err}
			}
		}()
	}
	go func() {
		wg.Wait()
	}()
}

func fetchSpan(client *orderly.Client, req helpers.CandleRequest) ([]models.OrderlyCandle, error) {
	var out []models.OrderlyCandle
	for _, segment := range candlespan.Split(req.StartMs, req.EndMs) {
		candles, err := executors.FetchCandles(client, req.Symbol, segment.Interval, segment.StartMs, segment.EndMs)
		if err != nil {
			return nil, err
		}
		barMs := excursion.IntervalMs(segment.Interval)
		for _, c := range candles {
			if excursion.Inside(c.StartTimestamp, barMs, req.StartMs, req.EndMs) {
				out = append(out, c)
			}
		}
	}
	return out, nil
}
