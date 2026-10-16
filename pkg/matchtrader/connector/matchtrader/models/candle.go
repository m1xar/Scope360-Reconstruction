package models

import (
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

type Candle struct {
	Pair     string
	Interval string
	OpenTime time.Time
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
}

type CandlesResponse struct {
	Symbol   string `json:"symbol"`
	Interval string `json:"interval"`
	Candles  []Bar  `json:"candles"`
}

type Bar struct {
	Time   Time      `json:"time"`
	Open   fx.Number `json:"open"`
	High   fx.Number `json:"high"`
	Low    fx.Number `json:"low"`
	Close  fx.Number `json:"close"`
	Volume fx.Number `json:"volume"`
}
