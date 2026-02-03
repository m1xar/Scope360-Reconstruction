package models

type FundingHistoryItem struct {
	Time  int64        `json:"time"`
	Hash  string       `json:"hash"`
	Delta FundingDelta `json:"delta"`
}

type FundingDelta struct {
	Type        string `json:"type"`
	Coin        string `json:"coin"`
	USDC        string `json:"usdc"`
	Szi         string `json:"szi"`
	FundingRate string `json:"fundingRate"`
	NSamples    *int   `json:"nSamples"`
}
