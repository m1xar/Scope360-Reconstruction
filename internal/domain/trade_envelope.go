package domain

import (
	"hyperliquid-trade-reconstructor/internal/hyperliquid/models"
)

type TradeEnvelope struct {
	Fills      []models.RawFill
	High       *float64
	Low        *float64
	StopLoss   *float64
	TakeProfit *float64
	Funding    float64
}
