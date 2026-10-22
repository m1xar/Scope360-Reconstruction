package domain

import (
	"time"
)

type FXPosition struct {
	ID               string
	Side             string
	Pair             string
	Symbol           string
	Amount           float64
	EntryPrice       float64
	ExitPrice        float64
	Pnl              float64
	NetPnl           float64
	Commission       float64
	Swap             float64
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
	CloseReason      string
	Status           *string
	CreatedAt        time.Time
	ClosedAt         *time.Time
	Orders           []FXOrder
	BalanceInit      float64
	// ContractSize is the number of base-asset units in one lot as reported by
	// the platform (EURUSD usually 100000, XAUUSD 100). It is not money per
	// point. nil means the platform did not report it for this symbol.
	ContractSize *float64
}
