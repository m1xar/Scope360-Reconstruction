package builders

import (
	"math"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
)

func BuildOpenPosition(pos models.OpenPosition) domain.OpenPosition {
	positionID, err := uuid.NewV7()
	if err != nil {
		positionID = uuid.Nil
	}

	return domain.OpenPosition{
		ID:           positionID,
		Pair:         helpers.NormalizePair(pos.InstId),
		Amount:       helpers.Round8(math.Abs(helpers.MustFloat(pos.Pos))),
		Side:         helpers.SideFromPosSide(pos.PosSide, pos.Pos),
		EntryPrice:   helpers.Round8(helpers.MustFloat(pos.AvgPx)),
		CurrentPrice: helpers.Round8(helpers.MustFloat(pos.MarkPx)),
		OpenTime:     helpers.TimeFromMs(pos.CTime),
	}
}
