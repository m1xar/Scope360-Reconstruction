package models

type Order struct {
	OrderID        string `json:"orderId"`
	TPSLID         string `json:"tpslId"`
	AlgoID         string `json:"algoId"`
	InstID         string `json:"instId"`
	PositionID     string `json:"positionId"`
	MarginMode     string `json:"marginMode"`
	PositionSide   string `json:"positionSide"`
	Side           string `json:"side"`
	OrderType      string `json:"orderType"`
	State          string `json:"state"`
	Price          string `json:"price"`
	Size           string `json:"size"`
	FilledSize     string `json:"filledSize"`
	AveragePrice   string `json:"averagePrice"`
	Fee            string `json:"fee"`
	Pnl            string `json:"pnl"`
	TpTriggerPrice string `json:"tpTriggerPrice"`
	SlTriggerPrice string `json:"slTriggerPrice"`
	TriggerPrice   string `json:"triggerPrice"`
	ActualPrice    string `json:"actualPrice"`
	CreateTime     string `json:"createTime"`
	UpdateTime     string `json:"updateTime"`
}
