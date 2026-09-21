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
	// KeepPartialBars disables dropping bars that straddle StartMs/EndMs
	// (needed when the caller wants the latest, still-open bar).
	KeepPartialBars bool
}

type CandleResponse struct {
	Candles []models.HyperliquidCandle
	Err     error
}
