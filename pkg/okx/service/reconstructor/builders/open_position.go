package builders

import (
	"math"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
)

func BuildOpenPositions(
	raw []models.OpenPosition,
	openFills map[string][]models.Fill,
	fills []models.Fill,
	ordersByID map[string]models.Order,
	instruments map[string]models.Instrument,
) []domain.OpenPosition {
	positions := make([]domain.OpenPosition, 0, len(raw))
	for _, r := range raw {
		if helpers.MustFloat(r.Pos) == 0 {
			continue
		}

		key := helpers.PositionKey(r.InstId, r.PosSide)
		own := openFills[key]
		if len(own) == 0 {
			own = fillsSince(fills, key, helpers.MustInt64(r.CTime))
		}

		positions = append(positions, buildOpenPosition(r, helpers.PartsFromFills(own, instruments), ordersByID, helpers.InstrumentFor(instruments, r.InstId)))
	}

	return positions
}

func fillsSince(fills []models.Fill, key string, fromMs int64) []models.Fill {
	out := make([]models.Fill, 0)
	for _, f := range fills {
		if helpers.FillKey(f) == key && helpers.MustInt64(f.Ts) >= fromMs {
			out = append(out, f)
		}
	}
	return out
}

func buildOpenPosition(
	pos models.OpenPosition,
	parts []helpers.FillPart,
	ordersByID map[string]models.Order,
	instrument models.Instrument,
) domain.OpenPosition {
	positionID, err := uuid.NewV7()
	if err != nil {
		positionID = uuid.Nil
	}

	return domain.OpenPosition{
		ID:           positionID,
		Pair:         helpers.NormalizePair(pos.InstId),
		Symbol:       pos.InstId,
		Amount:       helpers.Round8(math.Abs(helpers.ContractsToBase(pos.Pos, instrument))),
		Multiplier:   uint32(helpers.MustInt64(pos.Lever)),
		Side:         helpers.SideFromPosSide(pos.PosSide, pos.Pos),
		EntryPrice:   helpers.Round8(helpers.MustFloat(pos.AvgPx)),
		CurrentPrice: helpers.Round8(helpers.MustFloat(pos.MarkPx)),
		OpenTime:     helpers.TimeFromMs(pos.CTime),
		Orders:       buildOrders(parts, ordersByID, instrument, positionID),
	}
}
