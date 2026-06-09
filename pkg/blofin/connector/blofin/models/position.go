package models

type PositionHistory struct {
	HistoryID         string `json:"historyId"`
	InstID            string `json:"instId"`
	PositionID        string `json:"positionId"`
	MarginMode        string `json:"marginMode"`
	PositionSide      string `json:"positionSide"`
	Side              string `json:"side"`
	Leverage          string `json:"leverage"`
	OpenAveragePrice  string `json:"openAveragePrice"`
	CloseAveragePrice string `json:"closeAveragePrice"`
	MaxPositions      string `json:"maxPositions"`
	RealizedPnl       string `json:"realizedPnl"`
	Fee               string `json:"fee"`
	FundingFee        string `json:"fundingFee"`
	LiquidationPrice  string `json:"liquidationPrice"`
	CreateTime        string `json:"createTime"`
	UpdateTime        string `json:"updateTime"`
}

type OpenPosition struct {
	InstID           string `json:"instId"`
	PositionID       string `json:"positionId"`
	MarginMode       string `json:"marginMode"`
	PositionSide     string `json:"positionSide"`
	Side             string `json:"side"`
	Leverage         string `json:"leverage"`
	Positions        string `json:"positions"`
	AveragePrice     string `json:"averagePrice"`
	MarkPrice        string `json:"markPrice"`
	UnrealizedPnl    string `json:"unrealizedPnl"`
	LiquidationPrice string `json:"liquidationPrice"`
	CreateTime       string `json:"createTime"`
	UpdateTime       string `json:"updateTime"`
}
