package models

type CandleRows struct {
	Rows []OrderlyCandle `json:"rows"`
}

type OrderlyCandle struct {
	Open           float64 `json:"open"`
	Close          float64 `json:"close"`
	High           float64 `json:"high"`
	Low            float64 `json:"low"`
	Volume         float64 `json:"volume"`
	Amount         float64 `json:"amount"`
	Symbol         string  `json:"symbol"`
	Type           string  `json:"type"`
	StartTimestamp int64   `json:"start_timestamp"`
	EndTimestamp   int64   `json:"end_timestamp"`
}
