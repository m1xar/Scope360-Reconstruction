package reconstructor

import (
	"hyperliquid-trade-reconstructor/internal/hyperliquid/models"
	"strconv"
	"strings"
)

func mustFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func isOpen(dir string) bool {
	return dir == "Open Long" || dir == "Open Short"
}

func isClose(dir string) bool {
	return dir == "Close Long" || dir == "Close Short"
}

func isPerpDir(dir string) bool {
	return isOpen(dir) || isClose(dir)
}

func sideFromDir(dir string) string {
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
		v := mustFloat(ch.TriggerPx)
		if strings.Contains(ch.OrderType, "Stop") {
			sl = &v
		}
		if strings.Contains(ch.OrderType, "Take Profit") {
			tp = &v
		}
	}
	return
}
