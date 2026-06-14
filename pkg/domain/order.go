package domain

import (
	"time"

	"github.com/google/uuid"
)

type Order struct {
	ID              uuid.UUID
	PositionID      uuid.UUID
	ExchangeOrderID string
	Type            string
	Status          string
	Side            string
	Amount          float64
	AmountFilled    float64
	AveragePrice    float64
	StopPrice       float64
	OriginalPrice   float64
	UpdatedAt       time.Time
	Trade           Trade
}
