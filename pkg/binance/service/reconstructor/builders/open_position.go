package builders

import (
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/connector/binance/models"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func BuildOpenPositions(
	raw []models.PositionRisk,
	fills []models.Trade,
	ordersByID map[int64]models.Order,
	symbolCfg map[string]models.SymbolConfig,
) []domain.OpenPosition {
	open := helpers.OpenEpisodes(fills)

	positions := make([]domain.OpenPosition, 0, len(raw))
	for _, r := range raw {
		if helpers.MustFloat(r.PositionAmt) == 0 {
			continue
		}

		side := helpers.SideFromPositionRisk(r.PositionSide, r.PositionAmt)
		episode := open[helpers.EpisodeKey(r.Symbol, side)]
		leverage, _ := helpers.LeverageFor(symbolCfg, r.Symbol)

		positions = append(positions, buildOpenPosition(r, side, leverage, episode, ordersByID))
	}

	return positions
}

func buildOpenPosition(
	pos models.PositionRisk,
	side string,
	leverage float64,
	episode helpers.Episode,
	ordersByID map[int64]models.Order,
) domain.OpenPosition {
	positionID, err := uuid.NewV7()
	if err != nil {
		positionID = uuid.Nil
	}

	openTime := episode.OpenAt
	if openTime.IsZero() {
		openTime = helpers.TimeFromMs(pos.UpdateTime)
	}

	orders := buildOrders(episode.Parts, helpers.OrdersForEpisode(episode, ordersByID), positionID)
	if len(orders) == 0 {
		// Binance serves no fills for this position (history gap): stand in
		// a single opening order for the full size at the entry price.
		orders = []domain.Order{syntheticOpenOrder(pos, side, openTime, positionID)}
	}

	return domain.OpenPosition{
		ID:           positionID,
		Pair:         helpers.NormalizePair(pos.Symbol),
		Amount:       helpers.Round8(math.Abs(helpers.MustFloat(pos.PositionAmt))),
		Multiplier:   uint32(leverage),
		Side:         side,
		EntryPrice:   helpers.Round8(helpers.MustFloat(pos.EntryPrice)),
		CurrentPrice: helpers.Round8(helpers.MustFloat(pos.MarkPrice)),
		OpenTime:     openTime,
		Orders:       orders,
	}
}

func syntheticOpenOrder(pos models.PositionRisk, side string, at time.Time, positionID uuid.UUID) domain.Order {
	orderSide := "BUY"
	if side == "SHORT" {
		orderSide = "SELL"
	}
	amount := helpers.Round8(math.Abs(helpers.MustFloat(pos.PositionAmt)))
	price := helpers.Round8(helpers.MustFloat(pos.EntryPrice))
	orderID, err := uuid.NewV7()
	if err != nil {
		orderID = uuid.Nil
	}
	return domain.Order{
		ID:            orderID,
		PositionID:    positionID,
		Type:          "MARKET",
		Status:        "FILLED",
		Side:          orderSide,
		Amount:        amount,
		AmountFilled:  amount,
		AveragePrice:  price,
		OriginalPrice: price,
		UpdatedAt:     at,
		Trade: domain.Trade{
			OrderID: orderID,
			Side:    orderSide,
			Price:   price,
			Amount:  amount,
			DoneAt:  at,
		},
	}
}
