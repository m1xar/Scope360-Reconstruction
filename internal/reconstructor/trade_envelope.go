package reconstructor

import "hyperliquid-trade-reconstructor/internal/hyperliquid"

type TradeEnvelope struct {
	Fills      []hyperliquid.RawFill
	StopLoss   *float64
	TakeProfit *float64
}
