package reconstructor

import (
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
)

func ReconstructTrades(
	fills []models.RawFill,
	fundings []models.FundingHistoryItem,
	orderIdx helpers.OrderIndex,
	candleRequests chan<- helpers.CandleRequest,
	out chan<- envelope.TradeEnvelope,
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

		env := envelope.TradeEnvelope{
			Fills:      cp,
			StopLoss:   sl,
			TakeProfit: tp,
			Funding:    helpers.ExtractFunding(fundings, symbol, cp[0].Time, cp[len(cp)-1].Time),
			FillTypes:  fillTypes,
		}

		replyCh := make(chan helpers.CandleResponse, 1)
		candleRequests <- helpers.CandleRequest{
			Coin:     symbol,
			Interval: "1m",
			StartMs:  cp[0].Time,
			EndMs:    cp[len(cp)-1].Time,
			ReplyCh:  replyCh,
		}

		resp := <-replyCh
		if resp.Err == nil {
			env.High, env.Low = helpers.GetHighLowHyperliquid(resp.Candles)
		}

		out <- env
	}
}
