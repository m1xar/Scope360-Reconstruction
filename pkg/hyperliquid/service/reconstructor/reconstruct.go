package reconstructor

import (
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/executors"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
	models2 "github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/models"
	"math"
	"net/http"
	"strings"
	"time"
)

func ReconstructTrades(
	fills []models.RawFill,
	fundings []models.FundingHistoryItem,
	orderIdx helpers.OrderIndex,
	client *http.Client,
	endpoint string,
	out chan<- models2.TradeEnvelope,
) {
	usedFills := make(map[int64]struct{})
	const sizeEpsilon = 1e-9

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

			if math.Abs(size) < sizeEpsilon {
				cp := make([]models.RawFill, len(recon))
				copy(cp, recon)

				var sl, tp *float64
				if ordersAt, ok := orderIdx[f.Time]; ok {
					for _, ord := range ordersAt {
						if ord.Order.Coin != symbol || len(ord.Order.Children) == 0 {
							continue
						}
						sl, tp = helpers.ExtractTPSL(ord)
						break
					}
				}

				fillTypes := make(map[int64]string, len(cp))
				for _, fl := range cp {
					if ordersAt, ok := orderIdx[fl.Time]; ok {
						fillTypes[fl.Tid] = "MARKET"
						for _, ord := range ordersAt {
							if ord.Order.Coin != symbol {
								continue
							}
							ot := strings.ToLower(ord.Order.OrderType)

							switch {
							case strings.Contains(ot, "market"):
								fillTypes[fl.Tid] = "MARKET"
							case strings.Contains(ot, "limit"):
								fillTypes[fl.Tid] = "LIMIT"
							default:
								fillTypes[fl.Tid] = "MARKET"
							}

							break
						}
					}
				}

				env := models2.TradeEnvelope{
					Fills:      cp,
					StopLoss:   sl,
					TakeProfit: tp,
					Funding:    helpers.ExtractFunding(fundings, symbol, cp[0].Time, cp[len(cp)-1].Time),
					FillTypes:  fillTypes,
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
