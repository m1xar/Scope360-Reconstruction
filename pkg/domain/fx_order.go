package domain

import "time"

type FXOrder struct {
	ID            string
	PositionID    string
	Type          string
	Status        string
	Side          string
	Amount        float64
	AmountFilled  float64
	AveragePrice  float64
	StopPrice     float64
	OriginalPrice float64
	UpdatedAt     time.Time
	Trade         FXTrade
}
