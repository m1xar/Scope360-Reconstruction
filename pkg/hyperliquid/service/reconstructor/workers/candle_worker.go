package workers

import (
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/binance"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/executors"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
)

func StartCandleWorkers(
	client *resty.Client,
	endpoint string,
	requests <-chan helpers.CandleRequest,
	workerCount int,
) {
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for req := range requests {
				candles, err := fetchCandles(client, endpoint, req)
				req.ReplyCh <- helpers.CandleResponse{Candles: candles, Err: err}
			}
		}()
	}
	go func() {
		wg.Wait()
	}()
}

func fetchCandles(client *resty.Client, endpoint string, req helpers.CandleRequest) ([]models.HyperliquidCandle, error) {
	intervalMs, _ := helpers.IntervalToMs(req.Interval)
	oldestAllowedMs := time.Now().UnixMilli() - intervalMs*5000

	if req.StartMs < oldestAllowedMs {
		return binance.FetchFuturesKlinesPaged(
			client,
			req.Coin,
			req.Interval,
			req.StartMs,
			req.EndMs,
			499,
		)
	}

	return executors.FetchAllCandlesHyperliquid(
		client,
		endpoint,
		req.Coin,
		req.Interval,
		req.StartMs,
		req.EndMs,
	)
}
