package models

type Deposit struct {
	DepId string `json:"depId"`
	Amt   string `json:"amt"`
	Ccy   string `json:"ccy"`
	State string `json:"state"`
	Ts    string `json:"ts"`
}

type Withdrawal struct {
	WdId  string `json:"wdId"`
	Amt   string `json:"amt"`
	Fee   string `json:"fee"`
	Ccy   string `json:"ccy"`
	State string `json:"state"`
	Ts    string `json:"ts"`
}
