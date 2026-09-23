package builders

import (
	"math"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit/models"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func BuildOpenPositions(
	raw []models.Position,
	fills []helpers.Fill,
	ordersByID map[string]models.Order,
) []domain.OpenPosition {
	open := helpers.OpenEpisodes(fills)

	positions := make([]domain.OpenPosition, 0, len(raw))
	for _, r := range raw {
		if helpers.MustFloat(r.Size) == 0 {
			continue
		}

		side := helpers.SideFromPosition(r.Side)
		episode := open[helpers.EpisodeKey(r.Symbol, side)]

		positions = append(positions, buildOpenPosition(r, side, episode, ordersByID))
	}

	return positions
}

func buildOpenPosition(
	pos models.Position,
	side string,
	episode helpers.Episode,
	ordersByID map[string]models.Order,
) domain.OpenPosition {
	positionID, err := uuid.NewV7()
	if err != nil {
		positionID = uuid.Nil
	}

	openTime := episode.OpenAt
	if openTime.IsZero() {
		if ts := pos.OpenTime.Int64(); ts > 0 {
			openTime = helpers.TimeFromMs(ts)
		} else {
			openTime = helpers.TimeFromMs(pos.UpdatedTime.Int64())
		}
	}

	return domain.OpenPosition{
		ID:           positionID,
		Pair:         helpers.NormalizePair(pos.Symbol),
		Symbol:       pos.Symbol,
		Amount:       helpers.Round8(math.Abs(helpers.MustFloat(pos.Size))),
		Multiplier:   uint32(helpers.MustFloat(pos.Leverage)),
		Side:         side,
		EntryPrice:   helpers.Round8(helpers.MustFloat(pos.AvgPrice)),
		CurrentPrice: helpers.Round8(helpers.MustFloat(pos.MarkPrice)),
		OpenTime:     openTime,
		Orders: buildOrders(
			episode.Parts,
			helpers.OrdersForEpisode(episode, ordersByID),
			positionID,
		),
	}
}
