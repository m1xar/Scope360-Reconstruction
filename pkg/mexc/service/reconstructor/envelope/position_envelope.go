package envelope

import (
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
)

type PositionEnvelope struct {
	Symbol        string
	Side          string
	ContractSizes map[string]float64
	Parts         []helpers.FillPart
	OpenAt        time.Time
	CloseAt       time.Time
	OpenSign      float64
	Closed        bool
	Leverage      float64
	Isolated      bool
	StopLoss      *float64
	TakeProfit    *float64
	Funding       float64
	High          *float64
	Low           *float64
	CandlesLoaded bool
}
