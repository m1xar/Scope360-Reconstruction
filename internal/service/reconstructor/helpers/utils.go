package helpers

import (
	"errors"
	"hyperliquid-trade-reconstructor/internal/connector/hyperliquid/models"
	"strconv"
	"strings"
)

func MustFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func IsOpen(dir string) bool {
	return dir == "Open Long" || dir == "Open Short"
}

func IsClose(dir string) bool {
	return dir == "Close Long" || dir == "Close Short"
}

func isPerpDir(dir string) bool {
	return IsOpen(dir) || IsClose(dir)
}

func SideFromDir(dir string) string {
	if strings.Contains(dir, "Long") {
		return "Long"
	}
	return "Short"
}

func NormalizeFills(fills []models.RawFill) []models.RawFill {
	out := make([]models.RawFill, 0, len(fills))
	for _, f := range fills {
		if isPerpDir(f.Dir) {
			out = append(out, f)
		}
	}
	return out
}

func ExtractTPSL(o models.HistoricalOrder) (sl, tp *float64) {
	for _, ch := range o.Order.Children {
		v := MustFloat(ch.TriggerPx)
		if strings.Contains(ch.OrderType, "Stop") {
			sl = &v
		}
		if strings.Contains(ch.OrderType, "Take Profit") {
			tp = &v
		}
	}
	return
}

func ExtractFunding(
	fundings []models.FundingHistoryItem,
	coin string,
	from, to int64,
) float64 {

	out := 0.0

	for _, f := range fundings {
		if f.Delta.Coin != coin {
			continue
		}

		if f.Time < from || f.Time > to {
			continue
		}

		out += MustFloat(f.Delta.USDC)
	}

	return out
}

func IntervalToMs(interval string) (int64, error) {
	switch interval {
	case "1m":
		return 60_000, nil
	case "3m":
		return 3 * 60_000, nil
	case "5m":
		return 5 * 60_000, nil
	case "15m":
		return 15 * 60_000, nil
	case "30m":
		return 30 * 60_000, nil
	case "1h":
		return 60 * 60_000, nil
	case "2h":
		return 2 * 60 * 60_000, nil
	case "4h":
		return 4 * 60 * 60_000, nil
	case "8h":
		return 8 * 60 * 60_000, nil
	case "12h":
		return 12 * 60 * 60_000, nil
	case "1d":
		return 24 * 60 * 60_000, nil
	case "3d":
		return 3 * 24 * 60 * 60_000, nil
	case "1w":
		return 7 * 24 * 60 * 60_000, nil
	case "1M":
		return 30 * 24 * 60 * 60_000, nil
	default:
		return 0, errors.New("unsupported interval")
	}
}

func GetHighLowHyperliquid(candles []models.HyperliquidCandle) (high, low *float64) {
	if len(candles) == 0 {
		return nil, nil
	}

	h := MustFloat(candles[0].H)
	l := MustFloat(candles[0].L)

	for _, c := range candles {
		if MustFloat(c.H) > h {
			h = MustFloat(c.H)
		}
		if MustFloat(c.L) < l {
			l = MustFloat(c.L)
		}
	}

	return &h, &l
}
