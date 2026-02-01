package models

type ChildOrder struct {
	Coin             string       `json:"coin"`
	Side             string       `json:"side"`
	LimitPx          string       `json:"limitPx"`
	Sz               string       `json:"sz"`
	Oid              int64        `json:"oid"`
	Timestamp        int64        `json:"timestamp"`
	TriggerCondition string       `json:"triggerCondition"`
	IsTrigger        bool         `json:"isTrigger"`
	TriggerPx        string       `json:"triggerPx"`
	Children         []ChildOrder `json:"children"`
	IsPositionTpsl   bool         `json:"isPositionTpsl"`
	ReduceOnly       bool         `json:"reduceOnly"`
	OrderType        string       `json:"orderType"`
	OrigSz           string       `json:"origSz"`
	Tif              *string      `json:"tif"`
	Cloid            *string      `json:"cloid"`
}

type HistoricalOrder struct {
	Order struct {
		Coin      string       `json:"coin"`
		Side      string       `json:"side"`
		Oid       int64        `json:"oid"`
		Timestamp int64        `json:"timestamp"`
		OrderType string       `json:"orderType"`
		TriggerPx string       `json:"triggerPx"`
		Children  []ChildOrder `json:"children"`
	} `json:"order"`
	Status          string `json:"status"`
	StatusTimestamp int64  `json:"statusTimestamp"`
}
