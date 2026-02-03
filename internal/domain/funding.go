package domain

import (
	"time"

	"github.com/google/uuid"
)

type UserFunding struct {
	UserID    uuid.UUID
	KeyID     uint64
	Pair      string
	Amount    float64
	CreatedAt time.Time
}
