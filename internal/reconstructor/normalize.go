package reconstructor

import "hyperliquid-trade-reconstructor/internal/hyperliquid"

func NormalizeFills(fills []hyperliquid.RawFill) []hyperliquid.RawFill {
	out := make([]hyperliquid.RawFill, 0, len(fills))
	for _, f := range fills {
		if isPerpDir(f.Dir) {
			out = append(out, f)
		}
	}
	return out
}
