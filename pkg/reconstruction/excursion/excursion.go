// Package excursion computes MAE/MFE of a closed position from candles.
//
// Only bars that lie entirely inside the position's lifetime are used: the
// bars that contain the entry and the exit also carry prices from before the
// entry and after the exit (e.g. the wick after a stop-loss fill). The price
// range lost by dropping them is covered by clamping to net PnL and zero.
package excursion

import (
	"math"

	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/candlespan"
)

const (
	minuteMs = 60 * 1000
	dayMs    = 24 * 60 * minuteMs
)

// IntervalMs returns the duration of a candlespan interval, or 0 if unknown.
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

// Inside reports whether the bar [barOpenMs, barOpenMs+barMs) lies entirely
// within the position [openMs, closeMs].
func Inside(barOpenMs, barMs, openMs, closeMs int64) bool {
	return barMs > 0 && barOpenMs >= openMs && barOpenMs+barMs <= closeMs
}

// Compute returns MAE and MFE in net terms. high and low are the extremes of
// the bars inside the position, or nil when no bar lies fully inside it.
//
// The price excursion is shifted by the position's costs (netPnl - grossPnl:
// commission and funding), so the exit point equals netPnl exactly. The result
// is clamped so that MAE <= min(0, netPnl) and MFE >= max(0, netPnl).
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
