package helpers

import (
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
)

type CandleRequest struct {
	Coin     string
	Interval string
	StartMs  int64
	EndMs    int64
	ReplyCh  chan<- CandleResponse
}

type CandleResponse struct {
	Candles []models.HyperliquidCandle
	Err     error
}
