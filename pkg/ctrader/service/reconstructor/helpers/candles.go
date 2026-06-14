package helpers

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/models"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
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
		openRaw := lowRaw + float64(bar.GetDeltaOpen())
		closeRaw := lowRaw + float64(bar.GetDeltaClose())
		highRaw := lowRaw + float64(bar.GetDeltaHigh())
		out = append(out, models.Candle{
			Pair:     pair,
			Interval: interval,
			OpenTime: time.Unix(int64(bar.GetUtcTimestampInMinutes())*60, 0).UTC(),
			Open:     Round8(openRaw / priceScale),
			High:     Round8(highRaw / priceScale),
			Low:      Round8(lowRaw / priceScale),
			Close:    Round8(closeRaw / priceScale),
			Volume:   float64(bar.GetVolume()),
		})
	}
	return out
}

func SpotPrice(raw uint64) float64 {
	return Round8(float64(raw) / priceScale)
}

func CandleHighLow(candles []models.Candle) (high, low *float64) {
	if len(candles) == 0 {
		return nil, nil
	}
	h := candles[0].High
	l := candles[0].Low
	for _, candle := range candles[1:] {
		if candle.High > h {
			h = candle.High
		}
		if candle.Low < l {
			l = candle.Low
		}
	}
	return &h, &l
}

func ApplyFXMAEMFE(pos *domain.FXPosition, high, low *float64) {
	if pos == nil || high == nil || low == nil {
		return
	}
	amount := pos.Amount
	priceDelta := pos.ExitPrice - pos.EntryPrice
	if pos.Side == "SHORT" {
		priceDelta = pos.EntryPrice - pos.ExitPrice
	}
	if priceDelta != 0 && pos.Pnl != 0 {
		amount = math.Abs(pos.Pnl / priceDelta)
	}
	if pos.Side == "LONG" {
		maeVal := Round8(minFloat(0, (*low-pos.EntryPrice)*amount))
		mfeVal := Round8(maxFloat(0, (*high-pos.EntryPrice)*amount))
		pos.MAE = &maeVal
		pos.MFE = &mfeVal
		return
	}
	maeVal := Round8(minFloat(0, (pos.EntryPrice-*high)*amount))
	mfeVal := Round8(maxFloat(0, (pos.EntryPrice-*low)*amount))
	pos.MAE = &maeVal
	pos.MFE = &mfeVal
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
