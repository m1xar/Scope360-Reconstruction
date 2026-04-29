package models

type FillResponse struct {
	Result     string `json:"result"`
	Error      string `json:"error"`
	ServerTime string `json:"serverTime"`
	Fills      []Fill `json:"fills"`
}

type Fill struct {
	FillID   string        `json:"fill_id"`
	Symbol   string        `json:"symbol"`
	Side     string        `json:"side"`
	OrderID  string        `json:"order_id"`
	Size     FlexibleFloat `json:"size"`
	Price    FlexibleFloat `json:"price"`
	FillTime string        `json:"fillTime"`
	FillType string        `json:"fillType"`
}
