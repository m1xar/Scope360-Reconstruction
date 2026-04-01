package envelope

import (
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
)

type TradeEnvelope struct {
	Fills      []models.RawFill
	High       *float64
	Low        *float64
	StopLoss   *float64
	TakeProfit *float64
	Funding    float64
	FillTypes  map[int64]string
}
