package models

type Order struct {
	OrderId        int64   `json:"orderId"`
	Symbol         string  `json:"symbol"`
	PositionId     int64   `json:"positionId"`
	Price          float64 `json:"price"`
	Vol            float64 `json:"vol"`
	Leverage       int     `json:"leverage"`
	Side           int     `json:"side"` // 1=open long, 2=close short, 3=open short, 4=close long
	Category       int     `json:"category"`
	OrderType      int     `json:"orderType"` // 1=limit, 2=post-only, 3=IOC, 4=FOK, 5=market, 6=convert market
	DealAvgPrice   float64 `json:"dealAvgPrice"`
	DealVol        float64 `json:"dealVol"`
	OrderMargin    float64 `json:"orderMargin"`
	TakerFee       float64 `json:"takerFee"`
	MakerFee       float64 `json:"makerFee"`
	Profit         float64 `json:"profit"`
	FeeCurrency    string  `json:"feeCurrency"`
	OpenType       int     `json:"openType"` // 1=isolated, 2=cross
	State          int     `json:"state"`    // 1=uninformed, 2=uncompleted, 3=completed, 4=cancelled, 5=invalid
	ExternalOid    string  `json:"externalOid"`
	ErrorCode      int     `json:"errorCode"`
	UsedMargin     float64 `json:"usedMargin"`
	CreateTime     int64   `json:"createTime"`
	UpdateTime     int64   `json:"updateTime"`
	StopLossPrice  float64 `json:"stopLossPrice"`
	TakeProfitPrice float64 `json:"takeProfitPrice"`
}
