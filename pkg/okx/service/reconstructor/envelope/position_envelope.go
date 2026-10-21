package envelope

import (
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
)

type PositionEnvelope struct {
	InstID        string
	Side          string
	Instrument    models.Instrument
	Parts         []helpers.FillPart
	Orders        []models.Order
	OpenAt        time.Time
	CloseAt       time.Time
	PeakSize      float64
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
