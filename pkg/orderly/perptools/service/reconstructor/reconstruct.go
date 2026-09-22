package reconstructor

import (
	"math"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	connector "github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/models"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
)

const (
	defaultPositionWorkers = 8
	defaultCandleWorkers   = 4

	tradeChunk       = 14 * 24 * time.Hour
	maxTradeLookback = 3 * 365 * 24 * time.Hour
)

type TradeWalk struct {
	Groups [][]models.OrderlyTrade
	Trades []models.OrderlyTrade
}

func (w TradeWalk) EarliestOpenMs() int64 {
	earliest := int64(0)
	for _, group := range w.Groups {
		if len(group) == 0 {
			continue
		}
		ts := group[0].ExecutedTimestamp
		if ts == 0 {
			continue
		}
		if earliest == 0 || ts < earliest {
			earliest = ts
		}
	}
	return earliest
}

func collectClosedEpisodes(
	client *connector.Client,
	symbol string,
	openPositions []models.OrderlyPosition,
	cutoff *time.Time,
	reachMs int64,
) (TradeWalk, error) {
	segmenter := helpers.NewTradeSegmenter(openPositions)
	var walk TradeWalk

	if cutoff == nil {
		trades, err := executors.FetchAllTrades(client, symbol, 0, 0)
		if err != nil {
			return TradeWalk{}, err
		}
		walk.Groups = segmenter.PushOlderBatch(trades)
		walk.Trades = trades
		return walk, nil
	}

	var (
		nowMs     = time.Now().UnixMilli()
		cutoffMs  = cutoff.UnixMilli()
		floorMs   = nowMs - maxTradeLookback.Milliseconds()
		windowEnd = nowMs
	)

	for windowEnd >= floorMs {
		windowStart := windowEnd - tradeChunk.Milliseconds() + 1
		if windowStart < floorMs {
			windowStart = floorMs
		}

		trades, err := executors.FetchAllTrades(client, symbol, windowStart, windowEnd)
		if err != nil {
			return TradeWalk{}, err
		}
		walk.Groups = append(walk.Groups, segmenter.PushOlderBatch(trades)...)
		walk.Trades = append(walk.Trades, trades...)

		if windowStart <= floorMs {
			break
		}
		if windowStart <= cutoffMs && (reachMs == 0 || windowStart <= reachMs) && segmenter.Flat() {
			break
		}
		windowEnd = windowStart - 1
	}

	return walk, nil
}

func GroupsClosedAfter(groups [][]models.OrderlyTrade, cutoff *time.Time) [][]models.OrderlyTrade {
	if cutoff == nil {
		return groups
	}

	cutoffMs := cutoff.UnixMilli()
	kept := make([][]models.OrderlyTrade, 0, len(groups))
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		if group[len(group)-1].ExecutedTimestamp < cutoffMs {
			continue
		}
		kept = append(kept, group)
	}
	return kept
}

func ReconstructClosedPositions(
	client *connector.Client,
	symbol string,
	cutoff *time.Time,
) ([]domain.Position, error) {
	_ = symbol
	d, err := Load(client, cutoff, scope.Closed)
	if err != nil {
		return nil, err
	}
	return d.ClosedPositions()
}

func ReconstructTrades(
	groups [][]models.OrderlyTrade,
	fundings []models.OrderlyFunding,
	orderMap map[int64]models.OrderlyOrder,
	algoIdx helpers.AlgoOrderIndex,
	candleRequests chan<- helpers.CandleRequest,
	out chan<- envelope.TradeEnvelope,
) {
	type pendingCandle struct {
		env     envelope.TradeEnvelope
		replyCh chan helpers.CandleResponse
	}

	pending := make([]pendingCandle, 0, len(groups))

	for _, fills := range groups {
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
			Side:       helpers.GroupSide(fills),
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
			p.env.CandlesLoaded = true
		}
		out <- p.env
	}
}

func enrichOpenPositionOrders(trades []models.OrderlyTrade, orderMap map[int64]models.OrderlyOrder, positions []domain.OpenPosition) {
	for i := range positions {
		pos := &positions[i]
		openMs := pos.OpenTime.UnixMilli()
		matched := make([]models.OrderlyTrade, 0)
		for _, t := range trades {
			if helpers.NormalizeSymbol(t.Symbol) != pos.Pair {
				continue
			}
			if t.ExecutedTimestamp < openMs {
				continue
			}
			matched = append(matched, t)
		}
		pos.Orders = builders.BuildOpenOrdersFromTrades(matched, orderMap, pos.ID)
	}
}

func enrichPositionsWithRisk(resp *models.OrderlyPositionsResponse, positions []domain.Position) {
	if resp == nil || len(resp.Rows) == 0 || len(positions) == 0 {
		return
	}

	type riskInfo struct {
		leverage float64
		liq      float64
	}
	byPair := make(map[string]riskInfo, len(resp.Rows))
	for _, row := range resp.Rows {
		pair := strings.ToUpper(strings.TrimSpace(helpers.NormalizeSymbol(row.Symbol)))
		byPair[pair] = riskInfo{
			leverage: row.Leverage,
			liq:      row.EstLiqPrice,
		}
	}

	for i := range positions {
		pair := strings.ToUpper(strings.TrimSpace(positions[i].Pair))
		risk, ok := byPair[pair]
		if !ok {
			continue
		}
		if risk.leverage > 0 {
			positions[i].Multiplier = uint32(math.Round(risk.leverage))
		}
		if risk.liq > 0 {
			positions[i].LiquidationPrice = helpers.Round8(risk.liq)
		}
	}
}
