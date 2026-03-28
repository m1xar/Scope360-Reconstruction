package models

type Balance struct {
	TotalEq string          `json:"totalEq"`
	Details []BalanceDetail `json:"details"`
}

type BalanceDetail struct {
	Ccy      string `json:"ccy"`
	Eq       string `json:"eq"`
	CashBal  string `json:"cashBal"`
	AvailBal string `json:"availBal"`
}
