package helpers

import (
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
)

type CandleRequest struct {
	Symbol  string
	Bar     string
	StartMs int64
	EndMs   int64
	ReplyCh chan<- CandleResponse
}

type CandleResponse struct {
	Candles []models.Candle
	Err     error
}
