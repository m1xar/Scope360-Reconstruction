package workers

import (
	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/candlespan"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/excursion"
)

func StartCandleWorkers(
	client *resty.Client,
	baseURL string,
	requests <-chan helpers.CandleRequest,
	workerCount int,
) {
	for i := 0; i < workerCount; i++ {
		go func() {
			for req := range requests {
				candles, err := fetchSpan(client, baseURL, req)
				req.ReplyCh <- helpers.CandleResponse{Candles: candles, Err: err}
			}
		}()
	}
}

func fetchSpan(client *resty.Client, baseURL string, req helpers.CandleRequest) ([]models.Candle, error) {
	var out []models.Candle
	for _, segment := range candlespan.Split(req.StartMs, req.EndMs) {
		candles, err := executors.FetchCandles(client, baseURL, req.InstID, segment.Interval, segment.StartMs, segment.EndMs)
		if err != nil {
			return nil, err
		}
		barMs := excursion.IntervalMs(segment.Interval)
		for _, c := range candles {
			if excursion.Inside(helpers.MustInt64(c.Ts), barMs, req.StartMs, req.EndMs) {
				out = append(out, c)
			}
		}
	}
	return out, nil
}
