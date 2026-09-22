package domain

// Sync is everything a crypto connector reconstructs for an account in one
// pass: the raw exchange data is fetched once and every model is built from
// it. Each field carries exactly what the corresponding Get* call returns.
type Sync struct {
	Positions        []Position
	OpenPositions    []OpenPosition
	BalanceSnapshots []UserBalanceSnapshot
	CurrentBalance   float64
	Transactions     []Transaction
	Fundings         []UserFunding
}

// SyncFX is the FX (cTrader) counterpart of Sync.
type SyncFX struct {
	Positions        []FXPosition
	OpenPositions    []FXOpenPosition
	BalanceSnapshots []UserBalanceSnapshot
	AccountInfo      FXAccountInfo
	Transactions     []Transaction
}
