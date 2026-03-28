package models

type Order struct {
	InstId       string `json:"instId"`
	InstType     string `json:"instType"`
	OrdId        string `json:"ordId"`
	ClOrdId      string `json:"clOrdId"`
	OrdType      string `json:"ordType"`
	Side         string `json:"side"`
	PosSide      string `json:"posSide"`
	Px           string `json:"px"`
	Sz           string `json:"sz"`
	FillPx       string `json:"fillPx"`
	FillSz       string `json:"fillSz"`
	FillTime     string `json:"fillTime"`
	AvgPx        string `json:"avgPx"`
	AccFillSz    string `json:"accFillSz"`
	State        string `json:"state"`
	Lever        string `json:"lever"`
	TpTriggerPx  string `json:"tpTriggerPx"`
	TpOrdPx      string `json:"tpOrdPx"`
	SlTriggerPx  string `json:"slTriggerPx"`
	SlOrdPx      string `json:"slOrdPx"`
	Fee          string `json:"fee"`
	FeeCcy       string `json:"feeCcy"`
	Pnl          string `json:"pnl"`
	TdMode       string `json:"tdMode"`
	CTime        string `json:"cTime"`
	UTime        string `json:"uTime"`
}
