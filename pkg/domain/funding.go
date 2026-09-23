package domain

import (
	"time"
)

type UserFunding struct {
	Pair      string
	Symbol    string
	Amount    float64
	CreatedAt time.Time
}
