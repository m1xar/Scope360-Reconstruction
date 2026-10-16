package models

import "github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"

type PositionList struct {
	Positions []Position `json:"positions"`
}

type Position struct {
	Account          string    `json:"account"`
	Version          fx.Number `json:"version"`
	PositionCode     ID        `json:"positionCode"`
	Symbol           string    `json:"symbol"`
	Quantity         fx.Number `json:"quantity"`
	QuantityNotional fx.Number `json:"quantityNotional"`
	Side             string    `json:"side"`
	OpenTime         Time      `json:"openTime"`
	OpenPrice        fx.Number `json:"openPrice"`
	LastUpdateTime   Time      `json:"lastUpdateTime"`
	MarginRate       fx.Number `json:"marginRate"`
	TakeProfitPrice  fx.Number `json:"takeProfitPrice"`
	StopLossPrice    fx.Number `json:"stopLossPrice"`
}
