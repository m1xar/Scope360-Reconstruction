package domain

import "time"

type PositionDTO struct {
	ID             string
	Side           string
	Pair           string
	Amount         float64
	EntryPrice     float64
	ExitPrice      float64
	Pnl            float64
	NetPnl         float64
	Commission     float64
	MAE            float64
	MFE            float64
	TP             float64
	SL             float64
	Funding        float64
	Status         string
	CreatedAt      time.Time
	ClosedAt       *time.Time
	InitialBalance float64
}
