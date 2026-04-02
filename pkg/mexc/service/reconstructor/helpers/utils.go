package helpers

import (
	"math"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
)

func Round8(val float64) float64 {
	return math.Round(val*1e8) / 1e8
}

func TimeFromMs(ms int64) time.Time {
	return time.UnixMilli(ms).UTC()
}

func CutoffFromDays(days int) *time.Time {
	if days <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	return &cutoff
}

func NormalizePair(symbol string) string {
	return strings.ReplaceAll(symbol, "_", "")
}

func SideFromPositionType(posType int) string {
	if posType == 1 {
		return "LONG"
	}
	return "SHORT"
}

func OrderSideFromMEXC(side int) string {
	switch side {
	case 1, 2:
		return "BUY"
	case 3, 4:
		return "SELL"
	default:
		return "BUY"
	}
}

func OrderTypeFromMEXC(orderType int) string {
	switch orderType {
	case 1:
		return "LIMIT"
	case 2:
		return "POST_ONLY"
	case 3:
		return "IOC"
	case 4:
		return "FOK"
	case 5, 6:
		return "MARKET"
	default:
		return "MARKET"
	}
}

func IsOrderForLong(side int) bool {
	return side == 1 || side == 2
}

func GetHighLow(candles []models.Candle) (high, low *float64) {
	if len(candles) == 0 {
		return nil, nil
	}

	h := candles[0].High
	l := candles[0].Low

	for _, c := range candles[1:] {
		if c.High > h {
			h = c.High
		}
		if c.Low < l {
			l = c.Low
		}
	}

	return &h, &l
}
