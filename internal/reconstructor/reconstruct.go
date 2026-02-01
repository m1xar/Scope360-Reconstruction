package reconstructor

import "hyperliquid-trade-reconstructor/internal/hyperliquid"

func ReconstructTrades(fills []hyperliquid.RawFill, out chan<- []hyperliquid.RawFill) {
	usedFills := make(map[int64]struct{})

	for i := 0; i < len(fills); i++ {
		f := fills[i]

		if _, ok := usedFills[f.Tid]; ok {
			continue
		}
		if !isOpen(f.Dir) {
			continue
		}

		symbol := f.Coin
		side := sideFromDir(f.Dir)
		size := mustFloat(f.Sz)

		recon := []hyperliquid.RawFill{f}
		usedFills[f.Tid] = struct{}{}

		for j := i + 1; j < len(fills); j++ {
			n := fills[j]

			if _, ok := usedFills[n.Tid]; ok {
				continue
			}
			if n.Coin != symbol {
				continue
			}
			if sideFromDir(n.Dir) != side {
				continue
			}

			sz := mustFloat(n.Sz)

			if isOpen(n.Dir) {
				size += sz
			} else if isClose(n.Dir) {
				size -= sz
			}

			recon = append(recon, n)
			usedFills[n.Tid] = struct{}{}

			if size == 0 {
				cp := make([]hyperliquid.RawFill, len(recon))
				copy(cp, recon)
				out <- cp
				break
			}
		}
	}
}
