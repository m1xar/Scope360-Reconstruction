package models

type Balance struct {
	TS             string          `json:"ts"`
	TotalEquity    string          `json:"totalEquity"`
	IsolatedEquity string          `json:"isolatedEquity"`
	Details        []BalanceDetail `json:"details"`
}

type BalanceDetail struct {
	Currency        string `json:"currency"`
	Equity          string `json:"equity"`
	EquityUSD       string `json:"equityUsd"`
	Available       string `json:"available"`
	Frozen          string `json:"frozen"`
	OrderFrozen     string `json:"orderFrozen"`
	IsolatedEquity  string `json:"isolatedEquity"`
	CrossLiability  string `json:"crossLiability"`
	Bonus           string `json:"bonus"`
	AvailableEquity string `json:"availableEquity"`
}
