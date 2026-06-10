package models

type OrderlyAssetHistory struct {
	ID          string  `json:"id"`
	TxID        string  `json:"tx_id"`
	Side        string  `json:"side"`
	Token       string  `json:"token"`
	Amount      float64 `json:"amount"`
	Fee         float64 `json:"fee"`
	TransStatus string  `json:"trans_status"`
	CreatedTime int64   `json:"created_time"`
	UpdatedTime int64   `json:"updated_time"`
	ChainID     string  `json:"chain_id"`
	BlockTime   int64   `json:"block_time"`
}
