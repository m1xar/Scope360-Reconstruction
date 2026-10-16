package helpers

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

func Candles(pair, interval string, events []models.MarketEvent) []models.Candle {
	out := make([]models.Candle, 0, len(events))
	for _, e := range events {
		out = append(out, models.Candle{
			Pair:     pair,
			Interval: interval,
			OpenTime: e.Time.Time,
			Open:     e.Open.Float(),
			High:     e.High.Float(),
			Low:      e.Low.Float(),
			Close:    e.Close.Float(),
			Volume:   e.Volume.Float(),
		})
	}
	return out
}

func BarFetcher(ctx context.Context, c *connector.Client) fx.BarFetcher {
	var (
		mu     sync.Mutex
		denied error
	)
	return func(symbol, interval string, from, to time.Time) ([]fx.Bar, error) {
		mu.Lock()
		err := denied
		mu.Unlock()
		if err != nil {
			return nil, err
		}
		events, err := executors.FetchCandles(ctx, c, symbol, interval, from, to)
		if err != nil {
			var apiErr *connector.APIError
			if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 && apiErr.Status != http.StatusTooManyRequests {
				mu.Lock()
				denied = err
				mu.Unlock()
			}
			return nil, err
		}
		bars := make([]fx.Bar, 0, len(events))
		for _, e := range events {
			bars = append(bars, fx.Bar{OpenTime: e.Time.Time, High: e.High.Float(), Low: e.Low.Float()})
		}
		return bars, nil
	}
}
