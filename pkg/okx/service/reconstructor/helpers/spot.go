package helpers

import "strings"

type SpotLeg struct {
	Asset    string
	Quote    string
	InstId   string
	OrdId    string
	OrdType  string
	Side     string
	Qty      float64
	PxUSD    float64
	FeeBase  float64
	FeeUSD   float64
	Ts       int64
	Transfer bool
}

type SpotHolding struct {
	Asset    string
	Qty      float64
	AvgPx    float64
	PriceUSD float64
}

type PriceAt func(asset string, ts int64) (float64, bool)

const (
	OrderStatusSynthetic = "SYNTHETIC"
	OrderStatusTransfer  = "TRANSFER"
)

var stableAssets = map[string]bool{
	"USDT": true, "USDC": true, "USD": true, "USDG": true, "DAI": true,
	"TUSD": true, "FDUSD": true, "USDE": true, "PYUSD": true,
}

func IsStableAsset(ccy string) bool {
	return stableAssets[strings.ToUpper(ccy)]
}

func SpotFeeUSD(leg SpotLeg) float64 {
	return leg.FeeUSD + leg.FeeBase*leg.PxUSD
}
