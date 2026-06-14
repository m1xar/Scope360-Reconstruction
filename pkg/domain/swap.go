package domain

import "time"

type UserSwap struct {
	Pair      string
	Amount    float64
	CreatedAt time.Time
}
