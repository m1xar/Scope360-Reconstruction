package models

type OpenPositionsResponse struct {
	Result        string         `json:"result"`
	Error         string         `json:"error"`
	ServerTime    string         `json:"serverTime"`
	OpenPositions []OpenPosition `json:"openPositions"`
}

type OpenPosition struct {
	Side              string        `json:"side"`
	Symbol            string        `json:"symbol"`
	Price             FlexibleFloat `json:"price"`
	FillTime          string        `json:"fillTime"`
	Size              FlexibleFloat `json:"size"`
	UnrealizedFunding FlexibleFloat `json:"unrealizedFunding"`
	MaxFixedLeverage  FlexibleFloat `json:"maxFixedLeverage"`
	PNLCurrency       string        `json:"pnlCurrency"`
}
