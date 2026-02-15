package domain

import (
	"time"
)

type UserFunding struct {
	Pair      string
	Amount    float64
	CreatedAt time.Time
}
