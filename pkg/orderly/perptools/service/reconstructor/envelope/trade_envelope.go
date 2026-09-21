package envelope

import (
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/models"
)

type TradeEnvelope struct {
	Fills      []models.OrderlyTrade
	Side       string
	High       *float64
	Low        *float64
	StopLoss   *float64
	TakeProfit *float64
	Funding    float64
	FillTypes  map[int]string

	// CandlesLoaded is set when the candle fetch succeeded; High/Low stay nil
	// if no bar lies fully inside the position.
	CandlesLoaded bool
}
