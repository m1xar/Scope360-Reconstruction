package reconstructor

import (
	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/hyperliquid/executors"
	"hyperliquid-trade-reconstructor/internal/hyperliquid/models"
	"hyperliquid-trade-reconstructor/internal/reconstructor/helpers"
	"net/http"
	"time"
)

func ReconstructTrades(
	fills []models.RawFill,
	fundings []models.FundingHistoryItem,
	orderIdx helpers.OrderIndex,
	client *http.Client,
	endpoint string,
	out chan<- domain.TradeEnvelope,
) {
	usedFills := make(map[int64]struct{})

	for i := 0; i < len(fills); i++ {
		f := fills[i]

		if _, ok := usedFills[f.Tid]; ok || !helpers.IsOpen(f.Dir) {
			continue
		}

		symbol := f.Coin
		side := helpers.SideFromDir(f.Dir)
		size := helpers.MustFloat(f.Sz)

		recon := []models.RawFill{f}
		usedFills[f.Tid] = struct{}{}

		for j := i + 1; j < len(fills); j++ {
			n := fills[j]

			if _, ok := usedFills[n.Tid]; ok ||
				n.Coin != symbol ||
				helpers.SideFromDir(n.Dir) != side {
				continue
			}

			sz := helpers.MustFloat(n.Sz)

			if helpers.IsOpen(n.Dir) {
				size += sz
			} else if helpers.IsClose(n.Dir) {
				size -= sz
			}

			recon = append(recon, n)
			usedFills[n.Tid] = struct{}{}

			if size == 0 {
				cp := make([]models.RawFill, len(recon))
				copy(cp, recon)

				var sl, tp *float64
				if ord, ok := orderIdx[f.Time]; ok {
					sl, tp = helpers.ExtractTPSL(ord)
				}

				env := domain.TradeEnvelope{
					Fills:      cp,
					StopLoss:   sl,
					TakeProfit: tp,
					Funding:    helpers.ExtractFunding(fundings, symbol, cp[0].Time, cp[len(cp)-1].Time),
				}

				const maxAgeMinutes = int64(5000)
				maxAgeMs := maxAgeMinutes * 60 * 1000
				if time.Now().UnixMilli()-cp[0].Time < maxAgeMs {
					candles, err := executors.FetchAllCandlesHyperliquid(
						client,
						endpoint,
						symbol,
						"1m",
						cp[0].Time,
						cp[len(cp)-1].Time,
					)
					if err == nil {
						env.High, env.Low = helpers.GetHighLowHyperliquid(candles)
					}
				}

				out <- env
				break
			}
		}
	}
}
