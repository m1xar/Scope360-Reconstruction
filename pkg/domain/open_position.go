package domain

import (
	"time"

	"github.com/google/uuid"
)

type OpenPosition struct {
	ID           uuid.UUID
	Pair         string
	Amount       float64
	Side         string
	EntryPrice   float64
	CurrentPrice float64
	OpenTime     time.Time
	Orders       []Order
}
