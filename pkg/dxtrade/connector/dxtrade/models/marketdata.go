package models

import (
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

const (
	EventQuote    = "Quote"
	EventCandle   = "Candle"
	FormatCompact = "COMPACT"
)

type MarketDataRequest struct {
	Symbols    []string                 `json:"symbols"`
	EventTypes []MarketEventRequestType `json:"eventTypes"`
	Account    string                   `json:"account,omitempty"`
}

type MarketEventRequestType struct {
	Type       string `json:"type"`
	Format     string `json:"format,omitempty"`
	CandleType string `json:"candleType,omitempty"`
	FromTime   string `json:"fromTime,omitempty"`
	ToTime     string `json:"toTime,omitempty"`
	Count      int    `json:"count,omitempty"`
}

type MarketDataList struct {
	Events []MarketEvent `json:"events"`
}

type MarketEvent struct {
	Type       string    `json:"type"`
	Symbol     string    `json:"symbol"`
	CandleType string    `json:"candleType"`
	Bid        fx.Number `json:"bid"`
	Ask        fx.Number `json:"ask"`
	Open       fx.Number `json:"open"`
	High       fx.Number `json:"high"`
	Low        fx.Number `json:"low"`
	Close      fx.Number `json:"close"`
	Volume     fx.Number `json:"volume"`
	Time       Time      `json:"time"`
}

type Quote struct {
	Symbol string
	Bid    float64
	Ask    float64
	Time   time.Time
}

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
