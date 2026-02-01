package reconstructor

import (
	"hyperliquid-trade-reconstructor/internal/hyperliquid/models"
)

func ReconstructTrades(
	fills []models.RawFill,
	orderIdx OrderIndex,
	out chan<- TradeEnvelope,
) {
	usedFills := make(map[int64]struct{})

	for i := 0; i < len(fills); i++ {
		f := fills[i]

		if _, ok := usedFills[f.Tid]; ok || !isOpen(f.Dir) {
			continue
		}

		symbol := f.Coin
		side := sideFromDir(f.Dir)
		size := mustFloat(f.Sz)

		recon := []models.RawFill{f}
		usedFills[f.Tid] = struct{}{}

		for j := i + 1; j < len(fills); j++ {
			n := fills[j]

			if _, ok := usedFills[n.Tid]; ok ||
				n.Coin != symbol ||
				sideFromDir(n.Dir) != side {
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
				cp := make([]models.RawFill, len(recon))
				copy(cp, recon)

				var sl, tp *float64
				if ord, ok := orderIdx[f.Time]; ok {
					sl, tp = ExtractTPSL(ord)
				}

				out <- TradeEnvelope{
					Fills:      cp,
					StopLoss:   sl,
					TakeProfit: tp,
				}
				break
			}
		}
	}
}
