package reconstructor

import (
	"sort"
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	connector "github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly/models"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/service/reconstructor/workers"
)

const (
	defaultPositionWorkers = 8
	defaultCandleWorkers   = 4
)

func ReconstructClosedPositions(client *connector.Client, symbol string) ([]domain.Position, error) {
	trades, err := executors.FetchAllTrades(client, symbol, 0, 0)
	if err != nil {
		return nil, err
	}

	orders, err := executors.FetchFilledOrders(client, symbol, 0, 0)
	if err != nil {
		return nil, err
	}

	algoOrders, err := executors.FetchAlgoOrders(client, symbol, 0, 0)
	if err != nil {
		return nil, err
	}

	fundings, err := executors.FetchAllFunding(client, symbol, 0, 0)
	if err != nil {
		return nil, err
	}

	orderMap := helpers.BuildOrderMap(orders)
	algoIdx := helpers.BuildAlgoOrderIndex(algoOrders)

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, candleRequests, defaultCandleWorkers)

	envelopes := make(chan envelope.TradeEnvelope)
	positionsCh := make(chan domain.Position)

	go func() {
		ReconstructTrades(trades, fundings, orderMap, algoIdx, candleRequests, envelopes)
		close(envelopes)
		close(candleRequests)
	}()

	workers.StartPositionBuilders(envelopes, positionsCh, defaultPositionWorkers)

	positions := make([]domain.Position, 0)
	for pos := range positionsCh {
		positions = append(positions, pos)
	}

	sort.Slice(positions, func(i, j int) bool {
		iClosedAt := positions[i].ClosedAt
		jClosedAt := positions[j].ClosedAt
		if iClosedAt == nil && jClosedAt == nil {
			return i < j
		}
		if iClosedAt == nil {
			return false
		}
		if jClosedAt == nil {
			return true
		}
		return iClosedAt.Before(*jClosedAt)
	})

	return positions, nil
}

func ReconstructTrades(
	trades []models.OrderlyTrade,
	fundings []models.OrderlyFunding,
	orderMap map[int64]models.OrderlyOrder,
	algoIdx helpers.AlgoOrderIndex,
	candleRequests chan<- helpers.CandleRequest,
	out chan<- envelope.TradeEnvelope,
) {
	matches, _ := helpers.MatchFillGroups(trades, orderMap)

	type pendingCandle struct {
		env     envelope.TradeEnvelope
		replyCh chan helpers.CandleResponse
	}

	pending := make([]pendingCandle, 0, len(matches))

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
		pending = append(pending, pendingCandle{env: env, replyCh: replyCh})
	}

	for _, p := range pending {
		resp := <-p.replyCh
		if resp.Err == nil {
			p.env.High, p.env.Low = helpers.GetHighLow(resp.Candles)
		}
		out <- p.env
	}
}
