package excursion

import (
	"math"

	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/candlespan"
)

const (
	minuteMs = 60 * 1000
	dayMs    = 24 * 60 * minuteMs
)

func IntervalMs(interval string) int64 {
	switch interval {
	case candlespan.Minute:
		return minuteMs
	case candlespan.Day:
		return dayMs
	default:
		return 0
	}
}

func Inside(barOpenMs, barMs, openMs, closeMs int64) bool {
	return barMs > 0 && barOpenMs >= openMs && barOpenMs+barMs <= closeMs
}

func Compute(side string, entry, amount float64, high, low *float64, grossPnl, netPnl float64) (mae, mfe *float64) {
	var adverse, favorable float64
	if high != nil && low != nil {
		if side == "SHORT" {
			adverse = (entry - *high) * amount
			favorable = (entry - *low) * amount
		} else {
			adverse = (*low - entry) * amount
			favorable = (*high - entry) * amount
		}
	}

	costs := netPnl - grossPnl
	maeVal := round8(math.Min(math.Min(adverse+costs, netPnl), 0))
	mfeVal := round8(math.Max(math.Max(favorable+costs, netPnl), 0))
	return &maeVal, &mfeVal
}

func round8(v float64) float64 {
	return math.Round(v*1e8) / 1e8
}
