package models

type AccountLogResponse struct {
	AccountUID string       `json:"accountUid"`
	Logs       []AccountLog `json:"logs"`
}

type AccountLog struct {
	ID              int64         `json:"id"`
	UID             string        `json:"uid"`
	Date            string        `json:"date"`
	Asset           string        `json:"asset"`
	Contract        string        `json:"contract"`
	Info            string        `json:"info"`
	MarginAccount   string        `json:"margin_account"`
	Execution       string        `json:"execution"`
	BookingUID      string        `json:"booking_uid"`
	Fee             NullableFloat `json:"fee"`
	RealizedPnL     NullableFloat `json:"realized_pnl"`
	RealizedFunding NullableFloat `json:"realized_funding"`
	FundingRate     NullableFloat `json:"funding_rate"`
	TradePrice      NullableFloat `json:"trade_price"`
	MarkPrice       NullableFloat `json:"mark_price"`
	OldBalance      NullableFloat `json:"old_balance"`
	NewBalance      NullableFloat `json:"new_balance"`
}
