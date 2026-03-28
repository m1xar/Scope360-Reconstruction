package models

type OpenPosition struct {
	InstId      string `json:"instId"`
	InstType    string `json:"instType"`
	PosId       string `json:"posId"`
	PosSide     string `json:"posSide"`
	Pos         string `json:"pos"`
	AvgPx       string `json:"avgPx"`
	MarkPx      string `json:"markPx"`
	Upl         string `json:"upl"`
	Lever       string `json:"lever"`
	LiqPx       string `json:"liqPx"`
	MgnMode     string `json:"mgnMode"`
	CTime       string `json:"cTime"`
	UTime       string `json:"uTime"`
}
