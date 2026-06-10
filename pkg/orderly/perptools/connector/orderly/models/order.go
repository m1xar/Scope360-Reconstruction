package models

type OrderlyOrder struct {
	OrderID               int64   `json:"order_id"`
	UserID                int64   `json:"user_id"`
	Symbol                string  `json:"symbol"`
	Side                  string  `json:"side"`
	Type                  string  `json:"type"`
	Price                 float64 `json:"price"`
	Quantity              float64 `json:"quantity"`
	Amount                float64 `json:"amount"`
	ExecutedQuantity      float64 `json:"executed_quantity"`
	TotalExecutedQuantity float64 `json:"total_executed_quantity"`
	VisibleQuantity       float64 `json:"visible_quantity"`
	AverageExecutedPrice  float64 `json:"average_executed_price"`
	Status                string  `json:"status"`
	TotalFee              float64 `json:"total_fee"`
	FeeAsset              string  `json:"fee_asset"`
	ClientOrderID         string  `json:"client_order_id"`
	CreatedTime           int64   `json:"created_time"`
	UpdatedTime           int64   `json:"updated_time"`
	RealizedPnl           float64 `json:"realized_pnl"`
	ReduceOnly            bool    `json:"reduce_only"`
	OrderTag              string  `json:"order_tag"`
	MarginMode            string  `json:"margin_mode"`
}
