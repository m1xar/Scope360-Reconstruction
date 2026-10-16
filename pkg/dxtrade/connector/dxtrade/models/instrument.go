package models

import "github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"

type InstrumentList struct {
	Instruments []Instrument `json:"instruments"`
}

type Instrument struct {
	Symbol     string    `json:"symbol"`
	Type       string    `json:"type"`
	Currency   string    `json:"currency"`
	LotSize    fx.Number `json:"lotSize"`
	Multiplier fx.Number `json:"multiplier"`
}

type InstrumentDetailsList struct {
	InstrumentDetails []InstrumentDetails `json:"instrumentDetails"`
}

type InstrumentDetails struct {
	Account               string     `json:"account"`
	Type                  string     `json:"type"`
	Symbol                string     `json:"symbol"`
	TradingStatus         string     `json:"tradingStatus"`
	MinOrderSize          fx.Number  `json:"minOrderSize"`
	MinOrderSizeIncrement fx.Number  `json:"minOrderSizeIncrement"`
	MarginRate            MarginRate `json:"marginRate"`
	AssetClass            string     `json:"assetClass"`
}

type MarginRate struct {
	RateType string       `json:"rateType"`
	Tiers    []MarginTier `json:"tiers"`
}

type MarginTier struct {
	EndVolume fx.Number `json:"endVolume"`
	Value     fx.Number `json:"value"`
}
