package helpers

import (
	"math"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly/models"
)

func Round8(val float64) float64 {
	return math.Round(val*1e8) / 1e8
}

func NormalizeSymbol(symbol string) string {
	s := strings.TrimPrefix(symbol, "PERP_")
	s = strings.ReplaceAll(s, "_", "")
	return s
}

func SymbolFromPair(pair string) string {
	for _, quote := range []string{"USDC", "USDT"} {
		if strings.HasSuffix(pair, quote) {
			base := strings.TrimSuffix(pair, quote)
			return "PERP_" + base + "_" + quote
		}
	}
	return "PERP_" + pair + "_USDC"
}

func TradeSideToPositionSide(side string) string {
	if strings.EqualFold(side, "BUY") {
		return "LONG"
	}
	return "SHORT"
}

func CutoffFromDays(days int) *time.Time {
	if days <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	return &cutoff
}

func GetHighLow(candles []models.OrderlyCandle) (high, low *float64) {
	if len(candles) == 0 {
		return nil, nil
	}

	h := candles[0].High
	l := candles[0].Low

	for _, c := range candles {
		if c.High > h {
			h = c.High
		}
		if c.Low < l {
			l = c.Low
		}
	}

	return &h, &l
}
