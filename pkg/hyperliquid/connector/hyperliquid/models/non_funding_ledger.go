package models

type NonFundingLedgerUpdate struct {
	Time  int64                 `json:"time"`
	Hash  string                `json:"hash"`
	Delta NonFundingLedgerDelta `json:"delta"`
}

type NonFundingLedgerDelta struct {
	Type           string `json:"type"`
	USDC           string `json:"usdc"`
	Amount         string `json:"amount"`
	USDCValue      string `json:"usdcValue"`
	Token          string `json:"token"`
	SourceDex      string `json:"sourceDex"`
	DestinationDex string `json:"destinationDex"`
	ToPerp         *bool  `json:"toPerp"`
}
