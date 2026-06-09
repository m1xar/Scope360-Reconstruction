package helpers

import (
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
)

func SideFromRaw(side, positionSide string) string {
	switch strings.ToLower(strings.TrimSpace(positionSide)) {
	case "long":
		return "LONG"
	case "short":
		return "SHORT"
	}
	switch strings.ToLower(strings.TrimSpace(side)) {
	case "buy", "long":
		return "LONG"
	case "sell", "short":
		return "SHORT"
	default:
		return strings.ToUpper(strings.TrimSpace(side))
	}
}

func ContractValueByInstID(instruments []models.Instrument) map[string]float64 {
	values := make(map[string]float64, len(instruments))
	for _, inst := range instruments {
		values[inst.InstID] = MustFloat(inst.ContractValue)
	}
	return values
}

func PositionTimeMs(pos models.PositionHistory) (startMs, endMs int64) {
	return MustInt64(pos.CreateTime), MustInt64(pos.UpdateTime)
}
