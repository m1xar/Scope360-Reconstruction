package reconstructor

import (
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/binance"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/executors"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
	models2 "github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/models"
)

func ReconstructTrades(
	fills []models.RawFill,
	fundings []models.FundingHistoryItem,
	orderIdx helpers.OrderIndex,
	client *resty.Client,
	endpoint string,
	out chan<- models2.TradeEnvelope,
) {
	matches, _ := helpers.MatchFillGroups(fills)
	for _, match := range matches {
		cp := match.Fills
		if len(cp) == 0 {
			continue
		}

		symbol := cp[0].Coin
		first := cp[0]

		var sl, tp *float64
		if ordersAt, ok := orderIdx[first.Time]; ok {
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
			fillTypes[fl.Tid] = "MARKET"

			if ordersAt, ok := orderIdx[fl.Time]; ok {
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

		const interval = "1m"
		intervalMs, _ := helpers.IntervalToMs(interval)
		oldestAllowedMs := time.Now().UnixMilli() - intervalMs*5000

		var candles []models.HyperliquidCandle
		var err error
		if cp[0].Time < oldestAllowedMs {
			candles, err = binance.FetchFuturesKlinesPaged(
				client,
				symbol,
				interval,
				cp[0].Time,
				cp[len(cp)-1].Time,
				499,
			)
		} else {
			candles, err = executors.FetchAllCandlesHyperliquid(
				client,
				endpoint,
				symbol,
				interval,
				cp[0].Time,
				cp[len(cp)-1].Time,
			)
		}
		if err == nil {
			env.High, env.Low = helpers.GetHighLowHyperliquid(candles)
		}

		out <- env
	}
}
