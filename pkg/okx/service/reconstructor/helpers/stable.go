package helpers

import "strings"

var stableAssets = map[string]bool{
	"USDT": true, "USDC": true, "USD": true, "USDG": true, "DAI": true,
	"TUSD": true, "FDUSD": true, "USDE": true, "PYUSD": true,
}

func IsStableAsset(ccy string) bool {
	return stableAssets[strings.ToUpper(ccy)]
}
