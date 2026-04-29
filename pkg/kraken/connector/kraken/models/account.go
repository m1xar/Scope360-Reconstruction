package models

type AccountsResponse struct {
	Result     string             `json:"result"`
	Error      string             `json:"error"`
	ServerTime string             `json:"serverTime"`
	Accounts   map[string]Account `json:"accounts"`
}

type Account struct {
	Type            string        `json:"type"`
	Currency        string        `json:"currency"`
	PortfolioValue  FlexibleFloat `json:"portfolioValue"`
	BalanceValue    FlexibleFloat `json:"balanceValue"`
	AvailableMargin FlexibleFloat `json:"availableMargin"`
	Pnl             FlexibleFloat `json:"pnl"`
}
