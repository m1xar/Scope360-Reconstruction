package builders

import (
	"math"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func BuildOpenPositions(
	raw []models.OpenPosition,
	fills []models.Fill,
	ordersByID map[string]models.Order,
	instruments map[string]models.Instrument,
) []domain.OpenPosition {
	open := helpers.OpenEpisodes(fills, instruments)

	positions := make([]domain.OpenPosition, 0, len(raw))
	for _, r := range raw {
		if helpers.MustFloat(r.Positions) == 0 {
			continue
		}

		instrument := helpers.InstrumentFor(instruments, r.InstID)
		side := helpers.SideFromOpenPosition(r.PositionSide, r.Positions)
		episode := open[helpers.EpisodeKey(r.InstID, side)]

		positions = append(positions, buildOpenPosition(r, side, instrument, episode, ordersByID))
	}

	return positions
}

func buildOpenPosition(
	pos models.OpenPosition,
	side string,
	instrument models.Instrument,
	episode helpers.Episode,
	ordersByID map[string]models.Order,
) domain.OpenPosition {
	positionID, err := uuid.NewV7()
	if err != nil {
		positionID = uuid.Nil
	}

	return domain.OpenPosition{
		ID:           positionID,
		Pair:         helpers.NormalizePair(pos.InstID),
		Symbol:       pos.InstID,
		Amount:       helpers.Round8(math.Abs(helpers.ContractsToBase(pos.Positions, instrument))),
		Multiplier:   uint32(helpers.MustInt64(pos.Leverage)),
		Side:         side,
		EntryPrice:   helpers.Round8(helpers.MustFloat(pos.AveragePrice)),
		CurrentPrice: helpers.Round8(helpers.MustFloat(pos.MarkPrice)),
		OpenTime:     helpers.TimeFromMs(pos.CreateTime),
		Orders: buildOrders(
			episode.Parts,
			helpers.OrdersForEpisode(episode, ordersByID),
			instrument,
			positionID,
		),
	}
}
