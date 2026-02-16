package builders

import (
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/domain"
	"time"
)

func TradesFromPositions(positions []domain.Position) []domain.Trade { // моки собирать с ордеров, или с трейдов?
	trades := make([]domain.Trade, len(positions))
	index := 0
	for i := range positions {
		trades[index] = domain.Trade{
			OrderID:    positions[i].ID,
			Side:       positions[i].Side,
			Price:      positions[i].EntryPrice,
			Amount:     positions[i].Amount,
			Commission: positions[i].Commission,
			Profit:     positions[i].NetPnl,
			DoneAt:     time.Now().UTC(),
		}
		index++
	}
	return trades
}
