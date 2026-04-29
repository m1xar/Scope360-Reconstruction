package models

type TickersResponse struct {
	Result     string   `json:"result"`
	Error      string   `json:"error"`
	ServerTime string   `json:"serverTime"`
	Tickers    []Ticker `json:"tickers"`
}

type TickerResponse struct {
	Result     string `json:"result"`
	Error      string `json:"error"`
	ServerTime string `json:"serverTime"`
	Ticker     Ticker `json:"ticker"`
}

type Ticker struct {
	Symbol                string        `json:"symbol"`
	Pair                  string        `json:"pair"`
	Tag                   string        `json:"tag"`
	Last                  FlexibleFloat `json:"last"`
	MarkPrice             FlexibleFloat `json:"markPrice"`
	IndexPrice            FlexibleFloat `json:"indexPrice"`
	FundingRate           FlexibleFloat `json:"fundingRate"`
	FundingRatePrediction FlexibleFloat `json:"fundingRatePrediction"`
	OpenInterest          FlexibleFloat `json:"openInterest"`
	Bid                   FlexibleFloat `json:"bid"`
	Ask                   FlexibleFloat `json:"ask"`
}
