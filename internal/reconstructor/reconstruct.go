package reconstructor

import (
	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/hyperliquid/models"
)

func ReconstructTrades(
	fills []models.RawFill,
	fundings []models.FundingHistoryItem,
	orderIdx OrderIndex,
	out chan<- domain.TradeEnvelope,
) {
	usedFills := make(map[int64]struct{})

	for i := 0; i < len(fills); i++ {
		f := fills[i]

		if _, ok := usedFills[f.Tid]; ok || !IsOpen(f.Dir) {
			continue
		}

		symbol := f.Coin
		side := SideFromDir(f.Dir)
		size := MustFloat(f.Sz)

		recon := []models.RawFill{f}
		usedFills[f.Tid] = struct{}{}

		for j := i + 1; j < len(fills); j++ {
			n := fills[j]

			if _, ok := usedFills[n.Tid]; ok ||
				n.Coin != symbol ||
				SideFromDir(n.Dir) != side {
				continue
			}

			sz := MustFloat(n.Sz)

			if IsOpen(n.Dir) {
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

				out <- domain.TradeEnvelope{
					Fills:      cp,
					StopLoss:   sl,
					TakeProfit: tp,
					Funding:    extractFunding(fundings, symbol, cp[0].Time, cp[len(cp)-1].Time),
				}
				break
			}
		}
	}
}
