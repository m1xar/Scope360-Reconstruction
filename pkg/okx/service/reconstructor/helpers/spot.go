package helpers

import (
	"strconv"
	"strings"
)

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
	// Adjustment marks a synthetic execution derived from a SpotAdjustment;
	// OrdId then holds the adjustment Key.
	Adjustment bool
}

type SpotHolding struct {
	Asset    string
	Qty      float64
	AvgPx    float64
	PriceUSD float64
}

type PriceAt func(asset string, ts int64) (float64, bool)

// SpotAdjustment is the net quantity of an asset moved by activity that is not
// a cash spot order or a transfer (option premiums, deliveries, conversions),
// aggregated per UTC day; Ts is the start of that day in milliseconds.
type SpotAdjustment struct {
	Asset string
	Qty   float64
	Ts    int64
}

func (a SpotAdjustment) Key() string {
	return a.Asset + "@" + strconv.FormatInt(a.Ts, 10)
}

const SpotDayMs = int64(24 * 60 * 60 * 1000)

// SpotDayStart is the start of the UTC day of ts in milliseconds.
func SpotDayStart(ts int64) int64 { return ts - ts%SpotDayMs }

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
