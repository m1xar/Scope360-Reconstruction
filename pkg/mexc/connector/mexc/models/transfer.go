package models

type TransferRecord struct {
	Id         int64   `json:"id"`
	Txid       string  `json:"txid"`
	Currency   string  `json:"currency"`
	Amount     float64 `json:"amount"`
	Type       string  `json:"type"`  // IN or OUT
	State      string  `json:"state"` // WAIT, SUCCESS, FAILED
	CreateTime int64   `json:"createTime"`
	UpdateTime int64   `json:"updateTime"`
}
