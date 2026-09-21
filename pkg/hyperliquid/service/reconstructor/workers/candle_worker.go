package workers

import (
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/binance"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/candlespan"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/excursion"
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
				candles, err := fetchSpan(client, endpoint, req)
				req.ReplyCh <- helpers.CandleResponse{Candles: candles, Err: err}
			}
		}()
	}
	go func() {
		wg.Wait()
	}()
}

func fetchSpan(client *resty.Client, endpoint string, req helpers.CandleRequest) ([]models.HyperliquidCandle, error) {
	var out []models.HyperliquidCandle
	for _, segment := range candlespan.Split(req.StartMs, req.EndMs) {
		candles, err := fetchCandles(client, endpoint, req.Coin, segment.Interval, segment.StartMs, segment.EndMs)
		if err != nil {
			return nil, err
		}
		barMs := excursion.IntervalMs(segment.Interval)
		for _, c := range candles {
			if req.KeepPartialBars || excursion.Inside(c.T0, barMs, req.StartMs, req.EndMs) {
				out = append(out, c)
			}
		}
	}
	return out, nil
}

func fetchCandles(
	client *resty.Client,
	endpoint string,
	coin, interval string,
	startMs, endMs int64,
) ([]models.HyperliquidCandle, error) {
	intervalMs, _ := helpers.IntervalToMs(interval)
	oldestAllowedMs := time.Now().UnixMilli() - intervalMs*5000

	// Recent enough for Hyperliquid candle API — stay on HL.
	if startMs >= oldestAllowedMs {
		return executors.FetchAllCandlesHyperliquid(
			client,
			endpoint,
			coin,
			interval,
			startMs,
			endMs,
		)
	}

	// Older than HL window: pull Binance for the old slice, HL for the recent slice.
	var out []models.HyperliquidCandle

	binanceEnd := endMs
	if binanceEnd > oldestAllowedMs {
		binanceEnd = oldestAllowedMs - 1
	}
	if binanceEnd >= startMs {
		binanceCandles, err := binance.FetchFuturesKlinesPaged(
			client,
			coin,
			interval,
			startMs,
			binanceEnd,
			499,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, binanceCandles...)
	}

	if endMs >= oldestAllowedMs {
		hlStart := oldestAllowedMs
		if hlStart < startMs {
			hlStart = startMs
		}
		hlCandles, err := executors.FetchAllCandlesHyperliquid(
			client,
			endpoint,
			coin,
			interval,
			hlStart,
			endMs,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, hlCandles...)
	}

	return out, nil
}
