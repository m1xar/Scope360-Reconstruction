package helpers

import "github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"

type CandleRequest struct {
	InstID  string
	Bar     string
	StartMs int64
	EndMs   int64
	ReplyCh chan CandleResponse
}

type CandleResponse struct {
	Candles []models.Candle
	Err     error
}
