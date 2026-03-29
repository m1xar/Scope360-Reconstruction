package models

type OrderlyFunding struct {
	Symbol      string  `json:"symbol"`
	FundingRate float64 `json:"funding_rate"`
	FundingFee  float64 `json:"funding_fee"`
	MarkPrice   float64 `json:"mark_price"`
	PaymentType string  `json:"payment_type"`
	Status      string  `json:"status"`
	CreatedTime int64   `json:"created_time"`
	UpdatedTime int64   `json:"updated_time"`
	MarginMode  string  `json:"margin_mode"`
}
