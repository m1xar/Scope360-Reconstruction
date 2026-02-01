package domain

import (
	"time"

	"github.com/google/uuid"
)

type Order struct{}
type JSONMap map[string]any

type Position struct {
	ID               uuid.UUID
	UserID           uint64
	KeyID            uint64
	Side             string
	Pair             string
	Amount           float64
	EntryPrice       float64
	ExitPrice        float64
	Pnl              float64
	NetPnl           float64
	Commission       float64
	Funding          float64
	MAE              float64
	MFE              float64
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
	Editable         JSONMap
	BalanceInit      float64
}
