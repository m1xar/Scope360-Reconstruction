package domain

import "time"

type UserBalanceSnapshot struct {
	ResourceID uint64
	CreatedAt  time.Time
	Balance    float64
}
