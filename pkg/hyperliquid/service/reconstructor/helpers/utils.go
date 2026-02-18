package helpers

import (
	"encoding/json"
	"errors"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"math"
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
		return "BUY"
	}
	return "SELL"
}

func PositionSideFromDir(dir string) string {
	if strings.Contains(dir, "Long") {
		return "LONG"
	}
	return "SHORT"
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
		v := Round8(MustFloat(ch.TriggerPx))
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

func NormalizePortfolio(raw models.RawPortfolioResponse) (models.PortfolioResponse, error) {
	var result models.PortfolioResponse

	for _, entry := range raw {
		if len(entry) != 2 {
			continue
		}

		var period string
		if err := json.Unmarshal(entry[0], &period); err != nil {
			return nil, err
		}

		var rawData models.RawPeriodData
		if err := json.Unmarshal(entry[1], &rawData); err != nil {
			return nil, err
		}

		accountHistory, err := normalizeHistory(rawData.AccountValueHistory)
		if err != nil {
			return nil, err
		}

		pnlHistory, err := normalizeHistory(rawData.PnlHistory)
		if err != nil {
			return nil, err
		}

		result = append(result, models.PeriodEntry{
			Period: period,
			Data: models.PeriodData{
				AccountValueHistory: accountHistory,
				PnlHistory:          pnlHistory,
				Vlm:                 rawData.Vlm,
			},
		})
	}

	return result, nil
}

func normalizeHistory(raw [][]json.RawMessage) ([]models.HistoryPoint, error) {
	points := make([]models.HistoryPoint, 0, len(raw))

	for _, item := range raw {
		if len(item) != 2 {
			continue
		}

		var ts int64
		if err := json.Unmarshal(item[0], &ts); err != nil {
			return nil, err
		}

		var val string
		if err := json.Unmarshal(item[1], &val); err != nil {
			return nil, err
		}

		points = append(points, models.HistoryPoint{
			Timestamp: ts,
			Value:     val,
		})
	}

	return points, nil
}

func Round8(val float64) float64 {
	return math.Round(val*1e8) / 1e8
}
