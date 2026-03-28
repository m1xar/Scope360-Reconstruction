package domain

import (
	"time"

	"github.com/google/uuid"
)

type Trade struct {
	OrderID    uuid.UUID
	Side       string
	Price      float64
	Amount     float64
	Commission float64
	Profit     float64
	DoneAt     time.Time
}
