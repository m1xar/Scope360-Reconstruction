package models

type Fill struct {
	InstId   string `json:"instId"`
	InstType string `json:"instType"`
	OrdId    string `json:"ordId"`
	TradeId  string `json:"tradeId"`
	BillId   string `json:"billId"`
	Side     string `json:"side"`
	PosSide  string `json:"posSide"`
	FillSz   string `json:"fillSz"`
	FillPx   string `json:"fillPx"`
	Fee      string `json:"fee"`
	FillPnl  string `json:"fillPnl"`
	ExecType string `json:"execType"`
	Ts       string `json:"ts"`
}
