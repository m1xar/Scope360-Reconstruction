package envelope

import (
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/binance/connector/binance/models"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/service/reconstructor/helpers"
)

type PositionEnvelope struct {
	Symbol     string
	Side       string
	Parts      []helpers.FillPart
	Orders     map[int64]models.Order
	OpenAt     time.Time
	CloseAt    time.Time
	PeakSize   float64
	OpenSign   float64
	Closed     bool
	Leverage   float64
	Isolated   bool
	StopLoss   *float64
	TakeProfit *float64
	Funding    float64

	LiquidationFee float64
	High           *float64
	Low            *float64

	// CandlesLoaded is set when the candle fetch succeeded; High/Low stay nil
	// if no bar lies fully inside the position.
	CandlesLoaded bool
}
