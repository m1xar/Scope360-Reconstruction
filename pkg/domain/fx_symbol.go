package domain

const (
	ContractSizeMatchTrader = "matchtrader.contractSize"
	ContractSizeDXtrade     = "dxtrade.lotSize"
	ContractSizeCTrader     = "ctrader.lotSize/100"
)

type FXSymbolSpec struct {
	Symbol       string
	Pair         string
	ContractSize *float64
	Source       string
}
