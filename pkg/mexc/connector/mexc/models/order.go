package models

type Order struct {
	OrderId         string  `json:"orderId"`
	Symbol          string  `json:"symbol"`
	PositionId      int64   `json:"positionId"`
	Price           float64 `json:"price"`
	Vol             float64 `json:"vol"`
	Leverage        int     `json:"leverage"`
	Side            int     `json:"side"`
	Category        int     `json:"category"`
	OrderType       int     `json:"orderType"`
	DealAvgPrice    float64 `json:"dealAvgPrice"`
	DealVol         float64 `json:"dealVol"`
	OrderMargin     float64 `json:"orderMargin"`
	TakerFee        float64 `json:"takerFee"`
	MakerFee        float64 `json:"makerFee"`
	Profit          float64 `json:"profit"`
	FeeCurrency     string  `json:"feeCurrency"`
	OpenType        int     `json:"openType"`
	State           int     `json:"state"`
	ExternalOid     string  `json:"externalOid"`
	ErrorCode       int     `json:"errorCode"`
	UsedMargin      float64 `json:"usedMargin"`
	CreateTime      int64   `json:"createTime"`
	UpdateTime      int64   `json:"updateTime"`
	StopLossPrice   float64 `json:"stopLossPrice"`
	TakeProfitPrice float64 `json:"takeProfitPrice"`
}
