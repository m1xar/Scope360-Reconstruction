package helpers

import (
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly/models"
)

type CandleRequest struct {
	Symbol   string
	Interval string
	StartMs  int64
	EndMs    int64
	ReplyCh  chan<- CandleResponse
}

type CandleResponse struct {
	Candles []models.OrderlyCandle
	Err     error
}
