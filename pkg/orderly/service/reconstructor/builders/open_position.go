package builders

import (
	"math"
	"time"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/service/reconstructor/helpers"
)

func BuildOpenPositions(positions []models.OrderlyPosition) []domain.OpenPosition {
	out := make([]domain.OpenPosition, 0, len(positions))

	for _, p := range positions {
		if p.PositionQty == 0 {
			continue
		}

		side := "LONG"
		if p.PositionQty < 0 {
			side = "SHORT"
		}

		out = append(out, domain.OpenPosition{
			Pair:         helpers.NormalizeSymbol(p.Symbol),
			Amount:       helpers.Round8(math.Abs(p.PositionQty)),
			Side:         side,
			EntryPrice:   helpers.Round8(p.AverageOpenPrice),
			CurrentPrice: helpers.Round8(p.MarkPrice),
			OpenTime:     time.UnixMilli(p.Timestamp).UTC(),
		})
	}

	return out
}
