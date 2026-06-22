package domain

import "time"

type FXTrade struct {
	OrderID    string
	Side       string
	Price      float64
	Amount     float64
	Commission float64
	Profit     float64
	DoneAt     time.Time
}
