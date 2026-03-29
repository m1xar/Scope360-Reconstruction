package models

type OrderlyPositionsResponse struct {
	AccountValue float64           `json:"account_value"`
	Rows         []OrderlyPosition `json:"rows"`
}

type OrderlyPosition struct {
	Symbol           string  `json:"symbol"`
	PositionQty      float64 `json:"position_qty"`
	AverageOpenPrice float64 `json:"average_open_price"`
	MarkPrice        float64 `json:"mark_price"`
	UnsettledPnl     float64 `json:"unsettled_pnl"`
	EstLiqPrice      float64 `json:"est_liq_price"`
	CostPosition     float64 `json:"cost_position"`
	Leverage         float64 `json:"leverage"`
	MarginMode       string  `json:"margin_mode"`
	Timestamp        int64   `json:"timestamp"`
	IMR              float64 `json:"imr"`
	MMR              float64 `json:"mmr"`
	Pnl24H           float64 `json:"pnl_24_h"`
	Fee24H           float64 `json:"fee_24_h"`
}
