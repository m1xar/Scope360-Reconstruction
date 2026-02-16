package models

import "encoding/json"

type RawPortfolioResponse [][]json.RawMessage

type RawPeriodData struct {
	AccountValueHistory [][]json.RawMessage `json:"accountValueHistory"`
	PnlHistory          [][]json.RawMessage `json:"pnlHistory"`
	Vlm                 string              `json:"vlm"`
}
type PortfolioResponse []PeriodEntry

type PeriodEntry struct {
	Period string
	Data   PeriodData
}

type PeriodData struct {
	AccountValueHistory []HistoryPoint `json:"accountValueHistory"`
	PnlHistory          []HistoryPoint `json:"pnlHistory"`
	Vlm                 string         `json:"vlm"`
}

type HistoryPoint struct {
	Timestamp int64
	Value     string
}
