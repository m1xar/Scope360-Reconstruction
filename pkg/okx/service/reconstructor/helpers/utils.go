package helpers

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
)

func MustFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func MustInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func Round8(val float64) float64 {
	return math.Round(val*1e8) / 1e8
}

func SideFromDirection(dir string) string {
	switch strings.ToLower(dir) {
	case "long":
		return "LONG"
	case "short":
		return "SHORT"
	default:
		return strings.ToUpper(dir)
	}
}

func SideFromPosSide(posSide, pos string) string {
	switch strings.ToLower(posSide) {
	case "long":
		return "LONG"
	case "short":
		return "SHORT"
	case "net":
		if MustFloat(pos) >= 0 {
			return "LONG"
		}
		return "SHORT"
	default:
		return "LONG"
	}
}

func OrderTypeFromOKX(ordType string) string {
	switch strings.ToLower(ordType) {
	case "market", "optimal_limit_ioc":
		return "MARKET"
	case "limit", "post_only", "fok", "ioc":
		return "LIMIT"
	default:
		return "MARKET"
	}
}

func OrderSideFromOKX(side string) string {
	switch strings.ToLower(side) {
	case "buy":
		return "BUY"
	case "sell":
		return "SELL"
	default:
		return strings.ToUpper(side)
	}
}

func TimeFromMs(ms string) time.Time {
	return time.UnixMilli(MustInt64(ms)).UTC()
}

func CutoffFromDays(days int) *time.Time {
	if days <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	return &cutoff
}

func GetHighLow(candles []models.Candle) (high, low *float64) {
	if len(candles) == 0 {
		return nil, nil
	}

	h := MustFloat(candles[0].H)
	l := MustFloat(candles[0].L)

	for _, c := range candles {
		if v := MustFloat(c.H); v > h {
			h = v
		}
		if v := MustFloat(c.L); v < l {
			l = v
		}
	}

	return &h, &l
}

func IsFilled(order models.Order) bool {
	return MustFloat(order.AccFillSz) > 0
}
