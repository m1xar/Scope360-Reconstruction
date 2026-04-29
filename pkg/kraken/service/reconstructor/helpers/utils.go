package helpers

import (
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
)

var nonAlnum = regexp.MustCompile(`[^A-Z0-9]+`)

func Round8(val float64) float64 {
	return math.Round(val*1e8) / 1e8
}

func CutoffFromDays(days int) *time.Time {
	if days <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	return &cutoff
}

func ParseTime(raw string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
	}
	var lastErr error
	for _, layout := range layouts {
		t, err := time.Parse(layout, raw)
		if err == nil {
			return t.UTC(), nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}

func NormalizePairFallback(symbol string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	if s == "" {
		return s
	}
	parts := strings.Split(s, "_")
	if len(parts) >= 2 {
		return NormalizePairText(parts[1])
	}
	return NormalizePairText(s)
}

func NormalizePair(symbol string, pairBySymbol map[string]string) string {
	if pairBySymbol != nil {
		if pair := pairBySymbol[strings.ToUpper(symbol)]; pair != "" {
			return NormalizePairText(pair)
		}
	}
	return NormalizePairFallback(symbol)
}

func NormalizePairText(pair string) string {
	s := strings.ToUpper(strings.TrimSpace(pair))
	return nonAlnum.ReplaceAllString(s, "")
}

func SideSign(side string) float64 {
	if strings.EqualFold(side, "buy") {
		return 1
	}
	return -1
}

func PositionSideFromSign(sign float64) string {
	if sign >= 0 {
		return "LONG"
	}
	return "SHORT"
}

func OrderSide(side string) string {
	if strings.EqualFold(side, "buy") {
		return "BUY"
	}
	return "SELL"
}

func OrderType(fillType string) string {
	ft := strings.ToUpper(strings.TrimSpace(fillType))
	if ft == "" {
		return "MARKET"
	}
	return ft
}

func GetHighLow(candles []models.Candle) (high, low *float64) {
	if len(candles) == 0 {
		return nil, nil
	}

	h := candles[0].High.Float64()
	l := candles[0].Low.Float64()
	for _, c := range candles[1:] {
		if v := c.High.Float64(); v > h {
			h = v
		}
		if v := c.Low.Float64(); v < l {
			l = v
		}
	}
	return &h, &l
}
