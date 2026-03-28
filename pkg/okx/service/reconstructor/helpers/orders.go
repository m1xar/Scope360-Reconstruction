package helpers

import (
	"strings"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
)

func GroupOrdersByInst(orders []models.Order) map[string][]models.Order {
	idx := make(map[string][]models.Order)
	for _, o := range orders {
		idx[o.InstId] = append(idx[o.InstId], o)
	}
	return idx
}

func MatchOrdersToPosition(cp models.ClosedPosition, ordersByInst map[string][]models.Order) []models.Order {
	instOrders := ordersByInst[cp.InstId]
	if len(instOrders) == 0 {
		return nil
	}

	startMs := MustInt64(cp.CTime)
	endMs := MustInt64(cp.UTime)
	targetPosSide := directionToPosSide(cp.Direction)

	matched := make([]models.Order, 0)
	for _, ord := range instOrders {
		ordPosSide := strings.ToLower(ord.PosSide)
		if ordPosSide != "" && ordPosSide != "net" && targetPosSide != "" && ordPosSide != targetPosSide {
			continue
		}

		ordTime := MustInt64(ord.UTime)
		if ordTime >= startMs && ordTime <= endMs {
			matched = append(matched, ord)
		}
	}
	return matched
}

func directionToPosSide(direction string) string {
	switch strings.ToLower(direction) {
	case "long":
		return "long"
	case "short":
		return "short"
	default:
		return ""
	}
}
