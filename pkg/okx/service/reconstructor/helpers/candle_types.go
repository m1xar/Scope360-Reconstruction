package helpers

import (
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
)

type CandleRequest struct {
	InstId  string
	Bar     string
	StartMs int64
	EndMs   int64
	ReplyCh chan<- CandleResponse
}

type CandleResponse struct {
	Candles []models.Candle
	Err     error
}
