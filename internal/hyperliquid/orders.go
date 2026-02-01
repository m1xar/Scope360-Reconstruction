package hyperliquid

import "net/http"

// Это именно то, что лежит в children
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
	Children         []ChildOrder `json:"children"` // HL реально допускает вложенность
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
		Children  []ChildOrder `json:"children"` // ← ВОТ ЭТО КЛЮЧЕВОЕ ИСПРАВЛЕНИЕ
	} `json:"order"`
	Status          string `json:"status"`
	StatusTimestamp int64  `json:"statusTimestamp"`
}

func FetchHistoricalOrders(client *http.Client, endpoint, user string) ([]HistoricalOrder, error) {
	var out []HistoricalOrder
	err := doRequest(client, endpoint, map[string]any{
		"type": "historicalOrders",
		"user": user,
	}, &out)
	return out, err
}
