package models

type Bill struct {
	ID          string `json:"id"`
	Currency    string `json:"currency"`
	Amount      string `json:"amount"`
	Balance     string `json:"balance"`
	FromAccount string `json:"fromAccount"`
	ToAccount   string `json:"toAccount"`
	Type        string `json:"type"`
	TS          string `json:"ts"`
}
