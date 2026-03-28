package models

type ClosedPosition struct {
	InstId      string `json:"instId"`
	InstType    string `json:"instType"`
	MgnMode     string `json:"mgnMode"`
	PosId       string `json:"posId"`
	Direction   string `json:"direction"`
	OpenAvgPx   string `json:"openAvgPx"`
	CloseAvgPx  string `json:"closeAvgPx"`
	OpenMaxPos  string `json:"openMaxPos"`
	CloseTotalPos string `json:"closeTotalPos"`
	Pnl         string `json:"pnl"`
	RealizedPnl string `json:"realizedPnl"`
	Fee         string `json:"fee"`
	FundingFee  string `json:"fundingFee"`
	LiqPenalty  string `json:"liqPenalty"`
	Lever       string `json:"lever"`
	Type        string `json:"type"`
	CTime       string `json:"cTime"`
	UTime       string `json:"uTime"`
}
