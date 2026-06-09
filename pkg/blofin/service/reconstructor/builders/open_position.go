package builders

import (
	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func BuildOpenPosition(raw models.OpenPosition, contractValue float64) domain.OpenPosition {
	if contractValue <= 0 {
		contractValue = 1
	}
	id, _ := uuid.NewV7()
	rawAmount := helpers.MustFloat(raw.Positions)
	amount := rawAmount * contractValue
	if amount < 0 {
		amount = -amount
	}
	side := helpers.SideFromRaw(raw.Side, raw.PositionSide)
	if side == "" {
		side = "LONG"
		if rawAmount < 0 {
			side = "SHORT"
		}
	}
	return domain.OpenPosition{
		ID:           id,
		Pair:         helpers.NormalizePair(raw.InstID),
		Amount:       helpers.Round8(amount),
		Side:         side,
		EntryPrice:   helpers.Round8(helpers.MustFloat(raw.AveragePrice)),
		CurrentPrice: helpers.Round8(helpers.MustFloat(raw.MarkPrice)),
		OpenTime:     helpers.TimeFromMs(raw.CreateTime),
	}
}
