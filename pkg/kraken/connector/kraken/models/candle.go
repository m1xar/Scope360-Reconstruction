package models

type CandlesResponse struct {
	Candles     []Candle `json:"candles"`
	MoreCandles bool     `json:"more_candles"`
}

type Candle struct {
	Time   int64         `json:"time"`
	Open   FlexibleFloat `json:"open"`
	High   FlexibleFloat `json:"high"`
	Low    FlexibleFloat `json:"low"`
	Close  FlexibleFloat `json:"close"`
	Volume FlexibleFloat `json:"volume"`
}
