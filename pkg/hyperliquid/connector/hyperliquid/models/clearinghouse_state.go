package models

type ClearinghouseState struct {
	MarginSummary              MarginSummary   `json:"marginSummary"`
	CrossMarginSummary         MarginSummary   `json:"crossMarginSummary"`
	CrossMaintenanceMarginUsed string          `json:"crossMaintenanceMarginUsed"`
	Withdrawable               string          `json:"withdrawable"`
	AssetPositions             []AssetPosition `json:"assetPositions"`
	Time                       int64           `json:"time"`
}

type MarginSummary struct {
	AccountValue    string `json:"accountValue"`
	TotalNtlPos     string `json:"totalNtlPos"`
	TotalRawUsd     string `json:"totalRawUsd"`
	TotalMarginUsed string `json:"totalMarginUsed"`
}

type AssetPosition struct {
	Type     string   `json:"type"`
	Position Position `json:"position"`
}

type Position struct {
	Coin           string     `json:"coin"`
	Szi            string     `json:"szi"`
	Leverage       Leverage   `json:"leverage"`
	EntryPx        string     `json:"entryPx"`
	PositionValue  string     `json:"positionValue"`
	UnrealizedPnl  string     `json:"unrealizedPnl"`
	ReturnOnEquity string     `json:"returnOnEquity"`
	LiquidationPx  string     `json:"liquidationPx"`
	MarginUsed     string     `json:"marginUsed"`
	MaxLeverage    float64    `json:"maxLeverage"`
	CumFunding     CumFunding `json:"cumFunding"`
}

type Leverage struct {
	Type  string  `json:"type"`
	Value float64 `json:"value"`
}

type CumFunding struct {
	AllTime     string `json:"allTime"`
	SinceOpen   string `json:"sinceOpen"`
	SinceChange string `json:"sinceChange"`
}
