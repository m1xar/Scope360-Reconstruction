package domain

import "time"

type UserBalanceSnapshot struct {
	CreatedAt time.Time
	Balance   float64
}
