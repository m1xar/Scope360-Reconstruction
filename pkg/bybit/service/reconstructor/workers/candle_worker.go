package workers

import (
	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit/models"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/candlespan"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/excursion"
)

func StartCandleWorkers(
	client *resty.Client,
	requests <-chan helpers.CandleRequest,
	workerCount int,
) {
	for i := 0; i < workerCount; i++ {
		go func() {
			for req := range requests {
				candles, err := fetchSpan(client, req)
				req.ReplyCh <- helpers.CandleResponse{Candles: candles, Err: err}
			}
		}()
	}
}

func fetchSpan(client *resty.Client, req helpers.CandleRequest) ([]models.Candle, error) {
	var out []models.Candle

	startMs := executors.AlignToInterval(req.StartMs, candlespan.Minute)
	for _, segment := range candlespan.Split(startMs, req.EndMs) {
		candles, err := executors.FetchCandles(client, req.Symbol, segment.Interval, segment.StartMs, segment.EndMs)
		if err != nil {
			return nil, err
		}
		barMs := excursion.IntervalMs(segment.Interval)
		for _, c := range candles {
			if excursion.Inside(c.StartTime, barMs, req.StartMs, req.EndMs) {
				out = append(out, c)
			}
		}
	}
	return out, nil
}
