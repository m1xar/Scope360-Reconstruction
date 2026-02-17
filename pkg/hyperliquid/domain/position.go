package domain

import (
	"time"

	"github.com/google/uuid"
)

type Order struct {
	ID                uuid.UUID
	PositionID        uuid.UUID
	ExchangeOrderID   string
	Type              string
	OriginalOrderType string
	Status            string
	Side              string
	Reduce            bool
	Amount            float64
	AmountFilled      float64
	AveragePrice      float64
	StopPrice         float64
	OriginalPrice     float64
	UpdatedAt         time.Time
}

type Position struct {
	ID               uuid.UUID
	Side             string
	Pair             string
	Amount           float64
	EntryPrice       float64
	ExitPrice        float64
	Pnl              float64
	NetPnl           float64
	Commission       float64
	Funding          float64
	MAE              *float64
	MFE              *float64
	RR               *float64
	RRPlanned        *float64
	TP               *float64
	SL               *float64
	LiquidationPrice float64
	Multiplier       uint32
	Isolated         bool
	Closed           bool
	Status           *string
	CreatedAt        time.Time
	ClosedAt         *time.Time
	Orders           []Order
	Trades           []Trade
	BalanceInit      float64
}
