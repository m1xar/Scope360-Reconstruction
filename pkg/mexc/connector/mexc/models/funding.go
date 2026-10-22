package models

type FundingRecord struct {
	Id            int64   `json:"id"`
	Symbol        string  `json:"symbol"`
	PositionType  int     `json:"positionType"`
	PositionValue float64 `json:"positionValue"`
	Funding       float64 `json:"funding"`
	Rate          float64 `json:"rate"`
	SettleTime    int64   `json:"settleTime"`
}
