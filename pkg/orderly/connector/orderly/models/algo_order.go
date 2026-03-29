package models

type OrderlyAlgoOrder struct {
	AlgoOrderID           int64              `json:"algo_order_id"`
	AlgoStatus            string             `json:"algo_status"`
	AlgoType              string             `json:"algo_type"`
	Symbol                string             `json:"symbol"`
	Side                  string             `json:"side"`
	Type                  string             `json:"type"`
	Quantity              float64            `json:"quantity"`
	TriggerPrice          float64            `json:"trigger_price"`
	TriggerPriceType      string             `json:"trigger_price_type"`
	IsTriggered           bool               `json:"is_triggered"`
	CreatedTime           int64              `json:"created_time"`
	UpdatedTime           int64              `json:"updated_time"`
	TotalExecutedQuantity float64            `json:"total_executed_quantity"`
	TotalFee              float64            `json:"total_fee"`
	FeeAsset              string             `json:"fee_asset"`
	RealizedPnl           float64            `json:"realized_pnl"`
	RootAlgoOrderID       int64              `json:"root_algo_order_id"`
	ParentAlgoOrderID     int64              `json:"parent_algo_order_id"`
	RootAlgoOrderStatus   string             `json:"root_algo_order_status"`
	OrderTag              string             `json:"order_tag"`
	MarginMode            string             `json:"margin_mode"`
	ChildOrders           []OrderlyAlgoOrder `json:"child_orders"`
}
