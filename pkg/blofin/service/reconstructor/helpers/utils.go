package helpers

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
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

func NormalizePair(instID string) string {
	parts := strings.Split(instID, "-")
	if len(parts) >= 2 {
		return stripSpecial(parts[0] + parts[1])
	}
	return stripSpecial(instID)
}

func IsStableCurrency(currency string) bool {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "USDT", "USDC":
		return true
	default:
		return false
	}
}

func GetHighLow(candles []models.Candle) (high, low *float64) {
	if len(candles) == 0 {
		return nil, nil
	}

	h := MustFloat(candles[0].H)
	l := MustFloat(candles[0].L)
	for _, c := range candles[1:] {
		if v := MustFloat(c.H); v > h {
			h = v
		}
		if v := MustFloat(c.L); v < l {
			l = v
		}
	}
	return &h, &l
}

func stripSpecial(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
