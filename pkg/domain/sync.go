package domain

type Sync struct {
	Positions        []Position
	OpenPositions    []OpenPosition
	BalanceSnapshots []UserBalanceSnapshot
	CurrentBalance   float64
	Transactions     []Transaction
	Fundings         []UserFunding
}

type SyncFX struct {
	Positions        []FXPosition
	OpenPositions    []FXOpenPosition
	BalanceSnapshots []UserBalanceSnapshot
	AccountInfo      FXAccountInfo
	Transactions     []Transaction
}
