package domain

import "time"

type FXOpenPosition struct {
	ID           string
	Pair         string
	Amount       float64
	Side         string
	EntryPrice   float64
	CurrentPrice float64
	OpenTime     time.Time
	Orders       []FXOrder
}
