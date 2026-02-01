package reconstructor

import (
	"hyperliquid-trade-reconstructor/internal/hyperliquid"
	"strings"
)

type OrderIndex map[int64]hyperliquid.HistoricalOrder

func BuildOrderIndex(orders []hyperliquid.HistoricalOrder) OrderIndex {
	idx := make(OrderIndex)
	for _, o := range orders {
		idx[o.StatusTimestamp] = o
	}
	return idx
}

func ExtractTPSL(o hyperliquid.HistoricalOrder) (sl, tp *float64) {
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
