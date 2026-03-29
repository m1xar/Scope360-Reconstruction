package reconstructor

import (
	"strings"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/service/reconstructor/envelope"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/service/reconstructor/helpers"
)

func ReconstructTrades(
	trades []models.OrderlyTrade,
	fundings []models.OrderlyFunding,
	orderMap map[int64]models.OrderlyOrder,
	algoIdx helpers.AlgoOrderIndex,
	candleRequests chan<- helpers.CandleRequest,
	out chan<- envelope.TradeEnvelope,
) {
	matches, _ := helpers.MatchFillGroups(trades, orderMap)

	for _, match := range matches {
		fills := match.Fills
		if len(fills) == 0 {
			continue
		}

		symbol := fills[0].Symbol
		first := fills[0]
		last := fills[len(fills)-1]

		sl, tp := helpers.ExtractTPSL(algoIdx, symbol, first.ExecutedTimestamp, last.ExecutedTimestamp)

		fillTypes := make(map[int]string, len(fills))
		for _, f := range fills {
			fillTypes[f.ID] = "MARKET"

			if ord, ok := orderMap[f.OrderID]; ok {
				ot := strings.ToUpper(ord.Type)
				switch {
				case strings.Contains(ot, "LIMIT"):
					fillTypes[f.ID] = "LIMIT"
				default:
					fillTypes[f.ID] = "MARKET"
				}
			}
		}

		funding := helpers.ExtractFunding(fundings, symbol, first.ExecutedTimestamp, last.ExecutedTimestamp)

		env := envelope.TradeEnvelope{
			Fills:      fills,
			Side:       match.Side,
			StopLoss:   sl,
			TakeProfit: tp,
			Funding:    funding,
			FillTypes:  fillTypes,
		}

		replyCh := make(chan helpers.CandleResponse, 1)
		candleRequests <- helpers.CandleRequest{
			Symbol:   symbol,
			Interval: "1m",
			StartMs:  first.ExecutedTimestamp,
			EndMs:    last.ExecutedTimestamp,
			ReplyCh:  replyCh,
		}

		resp := <-replyCh
		if resp.Err == nil {
			env.High, env.Low = helpers.GetHighLow(resp.Candles)
		}

		out <- env
	}
}
