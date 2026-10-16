package domain

import "time"

type FXOpenPosition struct {
	ID            string
	Pair          string
	Symbol        string
	Amount        float64
	Side          string
	EntryPrice    float64
	CurrentPrice  float64
	UnrealizedPnl float64
	Commission    float64
	Swap          float64
	Margin        float64
	MAE           *float64
	MFE           *float64
	OpenTime      time.Time
	Orders        []FXOrder
}
