package models

import "github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"

type AccountMetricsList struct {
	Metrics []AccountMetrics `json:"metrics"`
}

type AccountMetrics struct {
	Account            string            `json:"account"`
	Version            fx.Number         `json:"version"`
	Equity             fx.Number         `json:"equity"`
	Balance            fx.Number         `json:"balance"`
	AvailableBalance   fx.Number         `json:"availableBalance"`
	Credit             fx.Number         `json:"credit"`
	OpenPl             fx.Number         `json:"openPl"`
	TotalPL            fx.Number         `json:"totalPL"`
	Margin             fx.Number         `json:"margin"`
	MarginFree         fx.Number         `json:"marginFree"`
	OpenPositionsCount fx.Number         `json:"openPositionsCount"`
	OpenOrdersCount    fx.Number         `json:"openOrdersCount"`
	Positions          []PositionMetrics `json:"positions"`
}

type PositionMetrics struct {
	PositionCode           ID        `json:"positionCode"`
	Symbol                 string    `json:"symbol"`
	Rate                   fx.Number `json:"rate"`
	Fpl                    fx.Number `json:"fpl"`
	ConvRate               fx.Number `json:"convRate"`
	BidOpen                fx.Number `json:"bidOpen"`
	AskOpen                fx.Number `json:"askOpen"`
	MarginRate             fx.Number `json:"marginRate"`
	Margin                 fx.Number `json:"margin"`
	Quantity               fx.Number `json:"quantity"`
	OpenPrice              fx.Number `json:"openPrice"`
	OpenPriceInstrumentCcy fx.Number `json:"openPriceInstrumentCcy"`
	FplInstrumentCcy       fx.Number `json:"fplInstrumentCcy"`
	InstrumentCurrency     string    `json:"instrumentCurrency"`
}
