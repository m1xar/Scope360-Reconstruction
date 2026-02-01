package reconstructor

import (
	"hyperliquid-trade-reconstructor/internal/hyperliquid/models"
)

type TradeEnvelope struct {
	Fills      []models.RawFill
	StopLoss   *float64
	TakeProfit *float64
}
