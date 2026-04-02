package builders

import (
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
)

func BuildOpenPosition(pos models.OpenPosition) domain.OpenPosition {
	return domain.OpenPosition{
		Pair:       helpers.NormalizePair(pos.Symbol),
		Amount:     helpers.Round8(pos.HoldVol),
		Side:       helpers.SideFromPositionType(pos.PositionType),
		EntryPrice: helpers.Round8(pos.OpenAvgPrice),
		OpenTime:   helpers.TimeFromMs(pos.CreateTime),
	}
}
