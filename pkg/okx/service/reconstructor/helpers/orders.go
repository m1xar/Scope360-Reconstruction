package helpers

import (
	"math"
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
)

func OrdersByID(orders []models.Order) map[string]models.Order {
	idx := make(map[string]models.Order, len(orders))
	for _, o := range orders {
		idx[o.OrdId] = o
	}
	return idx
}

func OrdersForParts(parts []FillPart, ordersByID map[string]models.Order) []models.Order {
	out := make([]models.Order, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		if _, ok := seen[part.Fill.OrdId]; ok {
			continue
		}
		if order, ok := ordersByID[part.Fill.OrdId]; ok {
			seen[part.Fill.OrdId] = struct{}{}
			out = append(out, order)
		}
	}
	return out
}

func OrderContext(orders []models.Order) (leverage float64, isolated bool) {
	for _, order := range orders {
		if lev := MustFloat(order.Lever); lev > leverage {
			leverage = lev
		}
		if strings.EqualFold(strings.TrimSpace(order.TdMode), "isolated") {
			isolated = true
		}
	}
	return leverage, isolated
}

func AllocateFillStats(part FillPart, instrument models.Instrument) (fee, pnl float64) {
	ratio := 1.0
	if fullSize := ContractsToBase(part.Fill.FillSz, instrument); fullSize > sizeEpsilon {
		ratio = part.Size / fullSize
		if ratio > 1 {
			ratio = 1
		}
	}
	fee = math.Abs(MustFloat(part.Fill.Fee)) * ratio
	pnl = MustFloat(part.Fill.FillPnl) * ratio
	return fee, pnl
}

func FundingForRange(bills []models.Bill, instID string, fromMs, toMs int64) float64 {
	var total float64
	for _, b := range bills {
		if b.Type != "8" || b.InstId != instID {
			continue
		}
		ts := MustInt64(b.Ts)
		if ts < fromMs || ts > toMs {
			continue
		}
		total += MustFloat(b.BalChg)
	}
	return Round8(total)
}

func ProtectiveLevels(orders []models.Order, side string) (tp, sl *float64) {
	entrySide := "buy"
	if side == "SHORT" {
		entrySide = "sell"
	}
	for _, pass := range []bool{true, false} {
		for _, ord := range orders {
			if !IsFilled(ord) || (pass && !strings.EqualFold(ord.Side, entrySide)) {
				continue
			}
			for _, attach := range ord.AttachAlgoOrds {
				applyLevel(&tp, attach.TpTriggerPx, attach.TpOrdPx)
				applyLevel(&sl, attach.SlTriggerPx, attach.SlOrdPx)
			}
			applyLevel(&tp, ord.TpTriggerPx, ord.TpOrdPx)
			applyLevel(&sl, ord.SlTriggerPx, ord.SlOrdPx)
		}
		if tp != nil || sl != nil {
			return tp, sl
		}
	}
	return tp, sl
}

func applyLevel(dst **float64, candidates ...string) {
	if *dst != nil {
		return
	}
	for _, raw := range candidates {
		if v := MustFloat(raw); v > 0 {
			rounded := Round8(v)
			*dst = &rounded
			return
		}
	}
}
