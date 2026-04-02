package models

type FundingRecord struct {
	Id            int64   `json:"id"`
	Symbol        string  `json:"symbol"`
	PositionType  int     `json:"positionType"` // 1=long, 2=short
	PositionValue float64 `json:"positionValue"`
	Funding       float64 `json:"funding"`
	Rate          float64 `json:"rate"`
	SettleTime    int64   `json:"settleTime"`
}
