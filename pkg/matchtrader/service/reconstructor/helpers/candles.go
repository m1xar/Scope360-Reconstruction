package helpers

import (
	"context"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

const CandleWorkers = 2

func BarFetcher(ctx context.Context, c *connector.Client) fx.BarFetcher {
	return func(symbol, interval string, from, to time.Time) ([]fx.Bar, error) {
		bars, err := executors.FetchCandles(ctx, c, symbol, interval, from, to)
		if err != nil {
			return nil, err
		}
		out := make([]fx.Bar, 0, len(bars))
		for _, b := range bars {
			out = append(out, fx.Bar{OpenTime: b.Time.Time, High: b.High.Float(), Low: b.Low.Float()})
		}
		return out, nil
	}
}

func Candles(pair, interval string, bars []models.Bar) []models.Candle {
	out := make([]models.Candle, 0, len(bars))
	for _, b := range bars {
		out = append(out, models.Candle{
			Pair: pair, Interval: interval, OpenTime: b.Time.Time,
			Open: b.Open.Float(), High: b.High.Float(), Low: b.Low.Float(), Close: b.Close.Float(), Volume: b.Volume.Float(),
		})
	}
	return out
}
