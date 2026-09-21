package helpers

import (
	"math"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/excursion"
)

func Round8(val float64) float64 {
	return math.Round(val*1e8) / 1e8
}

// CandleOpenMs returns the candle open time in ms; the kline API reports seconds.
func CandleOpenMs(c models.Candle) int64 {
	if c.Time < 1e12 {
		return c.Time * 1000
	}
	return c.Time
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

func BalanceWindowStart(positions []domain.Position, cutoff *time.Time) *time.Time {
	if cutoff == nil {
		return nil
	}

	start := *cutoff
	for _, pos := range positions {
		if pos.CreatedAt.Before(start) {
			start = pos.CreatedAt
		}
	}
	return &start
}

func IsStableCurrency(currency string) bool {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "USDT", "USDC":
		return true
	default:
		return false
	}
}

// ApplyMAEMFE sets MAE/MFE from the high/low of the bars inside the position;
// high/low are nil when no bar lies fully inside it.
func ApplyMAEMFE(pos *domain.Position, high, low *float64) {
	entry := pos.EntryPrice
	exit := pos.ExitPrice

	amount := pos.Amount
	priceDelta := exit - entry
	if pos.Side == "SHORT" {
		priceDelta = entry - exit
	}
	if priceDelta != 0 {
		amount = math.Abs(pos.Pnl / priceDelta)
	}

	pos.MAE, pos.MFE = excursion.Compute(pos.Side, entry, amount, high, low, pos.Pnl, pos.NetPnl)
}
