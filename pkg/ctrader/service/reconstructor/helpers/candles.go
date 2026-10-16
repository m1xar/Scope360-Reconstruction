package helpers

import (
	"context"
	"fmt"
	"strings"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/models"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

const priceScale = 100000.0

func TrendbarPeriod(interval string) (pb.ProtoOATrendbarPeriod, error) {
	switch strings.ToUpper(strings.TrimSpace(interval)) {
	case "1M", "M1":
		return pb.ProtoOATrendbarPeriod_M1, nil
	case "2M", "M2":
		return pb.ProtoOATrendbarPeriod_M2, nil
	case "3M", "M3":
		return pb.ProtoOATrendbarPeriod_M3, nil
	case "4M", "M4":
		return pb.ProtoOATrendbarPeriod_M4, nil
	case "5M", "M5":
		return pb.ProtoOATrendbarPeriod_M5, nil
	case "10M", "M10":
		return pb.ProtoOATrendbarPeriod_M10, nil
	case "15M", "M15":
		return pb.ProtoOATrendbarPeriod_M15, nil
	case "30M", "M30":
		return pb.ProtoOATrendbarPeriod_M30, nil
	case "1H", "H1":
		return pb.ProtoOATrendbarPeriod_H1, nil
	case "4H", "H4":
		return pb.ProtoOATrendbarPeriod_H4, nil
	case "12H", "H12":
		return pb.ProtoOATrendbarPeriod_H12, nil
	case "1D", "D1":
		return pb.ProtoOATrendbarPeriod_D1, nil
	case "1W", "W1":
		return pb.ProtoOATrendbarPeriod_W1, nil
	case "1MN", "MN1":
		return pb.ProtoOATrendbarPeriod_MN1, nil
	default:
		return 0, fmt.Errorf("unsupported cTrader trendbar interval %q", interval)
	}
}

func CandlesFromTrendbars(pair string, interval string, bars []*pb.ProtoOATrendbar) []models.Candle {
	out := make([]models.Candle, 0, len(bars))
	for _, bar := range bars {
		if bar == nil {
			continue
		}
		lowRaw := float64(bar.GetLow())
		out = append(out, models.Candle{
			Pair:     pair,
			Interval: interval,
			OpenTime: time.Unix(int64(bar.GetUtcTimestampInMinutes())*60, 0).UTC(),
			Open:     fx.Round8((lowRaw + float64(bar.GetDeltaOpen())) / priceScale),
			High:     fx.Round8((lowRaw + float64(bar.GetDeltaHigh())) / priceScale),
			Low:      fx.Round8(lowRaw / priceScale),
			Close:    fx.Round8((lowRaw + float64(bar.GetDeltaClose())) / priceScale),
			Volume:   float64(bar.GetVolume()),
		})
	}
	return out
}

func SpotPrice(raw uint64) float64 {
	return fx.Round8(float64(raw) / priceScale)
}

func BarFetcher(ctx context.Context, c *connector.Client, symbols Symbols) fx.BarFetcher {
	return func(symbol, interval string, from, to time.Time) ([]fx.Bar, error) {
		symbolID, ok := symbols.IDByName(symbol)
		if !ok {
			return nil, fmt.Errorf("ctrader symbol %q not found", symbol)
		}
		period, err := TrendbarPeriod(interval)
		if err != nil {
			return nil, err
		}
		bars, err := executors.FetchTrendbars(ctx, c, symbolID, period, from, to, 0)
		if err != nil {
			return nil, err
		}
		out := make([]fx.Bar, 0, len(bars))
		for _, candle := range CandlesFromTrendbars(symbol, interval, bars) {
			out = append(out, fx.Bar{OpenTime: candle.OpenTime, High: candle.High, Low: candle.Low})
		}
		return out, nil
	}
}
