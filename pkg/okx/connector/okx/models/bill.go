package models

type Bill struct {
	BillId   string `json:"billId"`
	InstId   string `json:"instId"`
	InstType string `json:"instType"`
	Type     string `json:"type"`
	SubType  string `json:"subType"`
	BalChg   string `json:"balChg"`
	Bal      string `json:"bal"`
	Sz       string `json:"sz"`
	Pnl      string `json:"pnl"`
	Fee      string `json:"fee"`
	Ccy      string `json:"ccy"`
	Ts       string `json:"ts"`
}
