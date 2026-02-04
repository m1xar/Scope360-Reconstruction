package domain

type OpenPosition struct {
	Pair             string
	Amount           float64
	Leverage         float64
	EntryPrice       float64
	Pnl              float64
	LiquidationPrice float64
	CurrentPrice     float64
}
