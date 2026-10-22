package domain

// Sources of FXSymbolSpec.ContractSize.
const (
	ContractSizeMatchTrader = "matchtrader.contractSize"
	ContractSizeDXtrade     = "dxtrade.lotSize"
	ContractSizeCTrader     = "ctrader.lotSize/100"
)

// FXSymbolSpec is the contract specification of one broker symbol, as
// returned by Dataset.ContractSizes() of the FX connectors. It is meant for
// backfilling FXPosition.ContractSize of positions stored before v0.4.4.
type FXSymbolSpec struct {
	Symbol string // broker symbol, e.g. "EURUSD.r" or "EUR/USD"
	Pair   string // normalized pair, the same value as FXPosition.Pair
	// ContractSize: base-asset units in one lot (not money per point);
	// nil if the platform did not report it.
	ContractSize *float64
	Source       string // one of the ContractSize* constants
}
