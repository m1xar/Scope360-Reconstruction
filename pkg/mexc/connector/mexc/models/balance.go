package models

type Asset struct {
	Currency        string  `json:"currency"`
	PositionMargin  float64 `json:"positionMargin"`
	FrozenBalance   float64 `json:"frozenBalance"`
	AvailableBalance float64 `json:"availableBalance"`
	CashBalance     float64 `json:"cashBalance"`
	Equity          float64 `json:"equity"`
	Unrealized      float64 `json:"unrealized"`
	Bonus           float64 `json:"bonus"`
}
