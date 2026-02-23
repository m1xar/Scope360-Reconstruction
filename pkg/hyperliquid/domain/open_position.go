package domain

import "time"

type OpenPosition struct {
	Pair         string
	Amount       float64
	Side         string
	EntryPrice   float64
	CurrentPrice float64
	OpenTime     time.Time
}
