package helpers

import (
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
)

const matchWindowMs = 10 * 60 * 1000

func GroupOrdersByInst(orders []models.Order) map[string][]models.Order {
	idx := make(map[string][]models.Order)
	for _, o := range orders {
		idx[o.InstID] = append(idx[o.InstID], o)
	}
	return idx
}

func MatchOrdersToPosition(pos models.PositionHistory, ordersByInst map[string][]models.Order) []models.Order {
	instOrders := ordersByInst[pos.InstID]
	if len(instOrders) == 0 {
		return nil
	}

	if pos.PositionID != "" {
		matched := make([]models.Order, 0)
		for _, ord := range instOrders {
			if ord.PositionID == pos.PositionID {
				matched = append(matched, ord)
			}
		}
		if len(matched) > 0 {
			return matched
		}
	}

	startMs := MustInt64(pos.CreateTime) - matchWindowMs
	endMs := MustInt64(pos.UpdateTime) + matchWindowMs
	targetPosSide := strings.ToLower(strings.TrimSpace(pos.PositionSide))
	targetSide := SideFromRaw(pos.Side, pos.PositionSide)

	matched := make([]models.Order, 0)
	for _, ord := range instOrders {
		ordTime := OrderTimeMs(ord)
		if ordTime < startMs || ordTime > endMs {
			continue
		}

		ordPosSide := strings.ToLower(strings.TrimSpace(ord.PositionSide))
		if targetPosSide != "" && targetPosSide != "net" && ordPosSide != "" && ordPosSide != "net" && ordPosSide != targetPosSide {
			continue
		}

		if ordPosSide == "" || targetPosSide == "" || targetPosSide == "net" || ordPosSide == "net" {
			if !orderSideFitsPosition(targetSide, ord.Side) {
				continue
			}
		}
		matched = append(matched, ord)
	}
	return matched
}

func OrderTimeMs(ord models.Order) int64 {
	if ord.UpdateTime != "" {
		return MustInt64(ord.UpdateTime)
	}
	return MustInt64(ord.CreateTime)
}

func OrderID(ord models.Order) string {
	if ord.OrderID != "" {
		return ord.OrderID
	}
	if ord.TPSLID != "" {
		return ord.TPSLID
	}
	return ord.AlgoID
}

func IsFilled(ord models.Order) bool {
	state := strings.ToLower(strings.TrimSpace(ord.State))
	if state != "" && state != "filled" && state != "partially_filled" && state != "partially-filled" && state != "partial-filled" {
		return false
	}
	return MustFloat(ord.FilledSize) > 0
}

func OrderSideFromRaw(side string) string {
	switch strings.ToLower(strings.TrimSpace(side)) {
	case "buy":
		return "BUY"
	case "sell":
		return "SELL"
	default:
		return strings.ToUpper(strings.TrimSpace(side))
	}
}

func OrderTypeFromRaw(ord models.Order) string {
	if ord.TPSLID != "" {
		return "TP_SL"
	}
	switch strings.ToLower(strings.TrimSpace(ord.OrderType)) {
	case "market":
		return "MARKET"
	case "limit", "post_only", "ioc", "fok":
		return "LIMIT"
	case "conditional", "trigger", "move_order_stop", "oco":
		return "STOP"
	default:
		if ord.TriggerPrice != "" || ord.TpTriggerPrice != "" || ord.SlTriggerPrice != "" {
			return "STOP"
		}
		return "MARKET"
	}
}

func StopPrice(ord models.Order) float64 {
	for _, raw := range []string{ord.SlTriggerPrice, ord.TpTriggerPrice, ord.TriggerPrice} {
		if v := MustFloat(raw); v > 0 {
			return Round8(v)
		}
	}
	return 0
}

func orderSideFitsPosition(positionSide, orderSide string) bool {
	side := strings.ToLower(strings.TrimSpace(orderSide))
	switch positionSide {
	case "LONG":
		return side == "buy" || side == "sell"
	case "SHORT":
		return side == "sell" || side == "buy"
	default:
		return true
	}
}
