package builders

import (
	"math"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/service/reconstructor/helpers"
)

func BuildOpenPosition(pos models.OpenPosition) domain.OpenPosition {
	return domain.OpenPosition{
		Pair:         pos.InstId,
		Amount:       helpers.Round8(math.Abs(helpers.MustFloat(pos.Pos))),
		Side:         helpers.SideFromPosSide(pos.PosSide, pos.Pos),
		EntryPrice:   helpers.Round8(helpers.MustFloat(pos.AvgPx)),
		CurrentPrice: helpers.Round8(helpers.MustFloat(pos.MarkPx)),
		OpenTime:     helpers.TimeFromMs(pos.CTime),
	}
}
