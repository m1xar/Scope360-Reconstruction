package builders

import (
	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
)

func BuildOpenPositions(
	raw []models.OpenPosition,
	openFills map[string][]models.Order,
	contractSizes map[string]float64,
) []domain.OpenPosition {
	episodes := helpers.OpenEpisodes(openFills, contractSizes)

	positions := make([]domain.OpenPosition, 0, len(raw))
	for _, pos := range raw {
		if pos.HoldVol <= 0 {
			continue
		}

		positionID, err := uuid.NewV7()
		if err != nil {
			positionID = uuid.Nil
		}
		episode := episodes[helpers.PositionKey(pos.Symbol, pos.PositionType)]

		positions = append(positions, domain.OpenPosition{
			ID:         positionID,
			Pair:       helpers.NormalizePair(pos.Symbol),
			Symbol:     pos.Symbol,
			Amount:     helpers.Round8(helpers.ContractVolumeToBase(pos.HoldVol, contractSizes[pos.Symbol])),
			Multiplier: uint32(pos.Leverage),
			Side:       helpers.SideFromPositionType(pos.PositionType),
			EntryPrice: helpers.Round8(pos.OpenAvgPrice),
			OpenTime:   helpers.TimeFromMs(pos.CreateTime),
			Orders:     buildOrders(episode.Parts, contractSizes, positionID),
		})
	}

	return positions
}
