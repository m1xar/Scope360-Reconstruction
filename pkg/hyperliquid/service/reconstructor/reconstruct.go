package reconstructor

import (
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/workers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
)

const (
	defaultPositionWorkers = 8
	defaultCandleWorkers   = 2
)

func ReconstructTrades(
	matches [][]models.RawFill,
	fundings []models.FundingHistoryItem,
	orderIdx helpers.OrderIndex,
	candleRequests chan<- helpers.CandleRequest,
	out chan<- envelope.TradeEnvelope,
) {
	type pendingCandle struct {
		env     envelope.TradeEnvelope
		replyCh chan helpers.CandleResponse
	}

	pending := make([]pendingCandle, 0, len(matches))

	for _, cp := range matches {
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
		pending = append(pending, pendingCandle{env: env, replyCh: replyCh})
	}

	for _, p := range pending {
		resp := <-p.replyCh
		if resp.Err == nil {
			p.env.High, p.env.Low = helpers.GetHighLowHyperliquid(resp.Candles)
			p.env.CandlesLoaded = true
		}
		out <- p.env
	}
}

type FillWalk struct {
	Segments [][]models.RawFill
	Fills    []models.RawFill
}

func (w FillWalk) EarliestMs() int64 {
	if len(w.Fills) == 0 {
		return 0
	}
	return w.Fills[0].Time
}

func CollectEpisodes(
	client *resty.Client,
	endpoint, user string,
	cutoff *time.Time,
) (FillWalk, error) {
	if cutoff == nil {
		fills, err := executors.FetchAllFills(client, endpoint, user)
		if err != nil {
			return FillWalk{}, err
		}
		fills = helpers.NormalizeFills(fills)
		return FillWalk{Segments: helpers.SegmentFills(fills), Fills: fills}, nil
	}

	var (
		segmenter = helpers.NewFillSegmenter()
		walk      FillWalk
		cutoffMs  = cutoff.UnixMilli()
	)

	recent, err := executors.FetchFillsRange(client, endpoint, user, cutoffMs, time.Now().UnixMilli())
	if err != nil {
		return FillWalk{}, err
	}
	recent = helpers.NormalizeFills(recent)
	walk.Segments = append(walk.Segments, segmenter.PushOlderBatch(recent)...)
	walk.Fills = recent

	fetch := func(startMs, endMs int64) ([]models.RawFill, error) {
		return executors.FetchFillsRange(client, endpoint, user, startMs, endMs)
	}
	earlier, segments, err := walkEarlier(fetch, segmenter, cutoffMs-1, time.Now().UnixMilli()-window.Retention.Milliseconds())
	if err != nil {
		return FillWalk{}, err
	}
	walk.Segments = append(walk.Segments, segments...)
	walk.Fills = append(earlier, walk.Fills...)

	return walk, nil
}

// earlierFillsWindow is the span of one backwards fills request before the
// cutoff; positions rarely straddle it by more than a few windows.
const earlierFillsWindow = 30 * 24 * time.Hour

// walkEarlier fetches fills older than endMs in windows, newest first, only
// while some position is still half walked (an episode that closed inside
// the cutoff window but opened before it). Every fill carries its
// startPosition, so the walker seeds itself and Flat() is exact. Returns the
// fetched fills oldest first and the closed episodes found.
func walkEarlier(
	fetch func(startMs, endMs int64) ([]models.RawFill, error),
	segmenter *helpers.FillSegmenter,
	endMs, floorMs int64,
) ([]models.RawFill, [][]models.RawFill, error) {
	var (
		earlier  []models.RawFill
		segments [][]models.RawFill
		synthTid int64
	)
	for _, span := range window.Backward(endMs, floorMs, earlierFillsWindow.Milliseconds()) {
		if segmenter.Flat() {
			break
		}
		fills, err := fetch(span.StartMs, span.EndMs)
		if err != nil {
			return nil, nil, err
		}
		fills = helpers.NormalizeFills(fills)
		// NormalizeFills numbers synthetic tids from -1 on every call; keep
		// them unique across windows.
		for i := range fills {
			if fills[i].Tid < 0 {
				synthTid--
				fills[i].Tid = synthTid
			}
		}
		segments = append(segments, segmenter.PushOlderBatch(fills)...)
		earlier = append(fills, earlier...)
	}
	return earlier, segments, nil
}

func FillsSince(
	client *resty.Client,
	endpoint, user string,
	cutoff *time.Time,
) ([]models.RawFill, error) {
	if cutoff == nil {
		fills, err := executors.FetchAllFills(client, endpoint, user)
		if err != nil {
			return nil, err
		}
		return helpers.NormalizeFills(fills), nil
	}

	fills, err := executors.FetchFillsRange(
		client, endpoint, user,
		cutoff.UnixMilli(), time.Now().UnixMilli(),
	)
	if err != nil {
		return nil, err
	}
	return helpers.NormalizeFills(fills), nil
}

func ReconstructClosedPositions(
	client *resty.Client,
	endpoint, user string,
	cutoff *time.Time,
) ([]domain.Position, error) {
	walk, err := CollectEpisodes(client, endpoint, user, cutoff)
	if err != nil {
		return nil, err
	}

	segments := helpers.EpisodesClosedAfter(walk.Segments, cutoff)
	if len(segments) == 0 {
		return []domain.Position{}, nil
	}

	orders, err := executors.FetchHistoricalOrders(client, endpoint, user)
	if err != nil {
		return nil, err
	}

	rawFundings, err := executors.FetchAllFunding(client, endpoint, user, walk.EarliestMs())
	if err != nil {
		return nil, err
	}

	rawPortfolio, err := executors.FetchPortfolioState(client, endpoint, user)
	if err != nil {
		return nil, err
	}

	portfolio, err := helpers.NormalizePortfolio(rawPortfolio)
	if err != nil {
		return nil, err
	}

	orderIdx := helpers.BuildOrderIndex(orders)

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, endpoint, candleRequests, defaultCandleWorkers)

	envelopes := make(chan envelope.TradeEnvelope)
	positionsCh := make(chan domain.Position)

	go func() {
		ReconstructTrades(segments, rawFundings, orderIdx, candleRequests, envelopes)
		close(envelopes)
		close(candleRequests)
	}()

	workers.StartPositionBuilders(envelopes, positionsCh, defaultPositionWorkers)

	positions := make([]domain.Position, 0)
	for pos := range positionsCh {
		positions = append(positions, pos)
	}

	sort.Slice(positions, func(i, j int) bool {
		return positions[i].ClosedAt.Before(*positions[j].ClosedAt)
	})

	balanceSnapshots := builders.BuildUserBalanceSnapshotsFromPortfolio(portfolio)
	helpers.ReconstructBalancesFromRawFills(walk.Fills, &balanceSnapshots)
	helpers.AttachBalanceInit(&positions, balanceSnapshots)
	positions = helpers.FilterPositionsByClosedAt(positions, cutoff)
	for i := range positions {
		positions[i].Pair = helpers.NormalizeContractName(positions[i].Pair)
	}
	return positions, nil
}

func FindClosedPosition(
	client *resty.Client,
	endpoint, user, pair string,
	openedAt time.Time,
	side string,
) (*domain.Position, error) {
	pair = helpers.NormalizeContractName(pair)
	coin := helpers.CoinFromPair(pair)

	// The position opened at openedAt, so nothing before it is needed.
	sinceMs := openedAt.UnixMilli()
	allFills, err := executors.FetchFillsRange(client, endpoint, user, sinceMs, time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}

	allFills = helpers.NormalizeFills(allFills)
	fills := helpers.FilterFillsByCoinAndTime(allFills, coin, openedAt)

	orders, err := executors.FetchHistoricalOrders(client, endpoint, user)
	if err != nil {
		return nil, err
	}

	rawFundings, err := executors.FetchAllFunding(client, endpoint, user, sinceMs)
	if err != nil {
		return nil, err
	}

	orderIdx := helpers.BuildOrderIndex(orders)

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, endpoint, candleRequests, defaultCandleWorkers)

	envelopes := make(chan envelope.TradeEnvelope)
	positionsCh := make(chan domain.Position)

	go func() {
		ReconstructTrades(helpers.SegmentFills(fills), rawFundings, orderIdx, candleRequests, envelopes)
		close(envelopes)
		close(candleRequests)
	}()

	workers.StartPositionBuilders(envelopes, positionsCh, defaultPositionWorkers)

	var result *domain.Position
	for pos := range positionsCh {
		if result != nil {
			continue
		}
		pos.Pair = helpers.NormalizeContractName(pos.Pair)
		if pos.Pair == pair && pos.Side == side && pos.CreatedAt.Equal(openedAt) {
			matched := pos
			result = &matched
		}
	}

	if result != nil {
		rawPortfolio, err := executors.FetchPortfolioState(client, endpoint, user)
		if err != nil {
			return nil, err
		}
		portfolio, err := helpers.NormalizePortfolio(rawPortfolio)
		if err != nil {
			return nil, err
		}
		snapshots := builders.BuildUserBalanceSnapshotsFromPortfolio(portfolio)
		helpers.ReconstructBalancesFromRawFills(allFills, &snapshots)
		positions := []domain.Position{*result}
		helpers.AttachBalanceInit(&positions, snapshots)
		result = &positions[0]
	}

	return result, nil
}

func ReconstructOpenPositions(
	client *resty.Client,
	endpoint, user string,
) ([]domain.OpenPosition, error) {
	fills, err := executors.FetchAllFills(client, endpoint, user)
	if err != nil {
		return nil, err
	}

	fills = helpers.NormalizeFills(fills)

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, endpoint, candleRequests, defaultCandleWorkers)

	openPositions := builders.BuildOpenPositionsFromFills(candleRequests, fills)
	close(candleRequests)

	for i := range openPositions {
		openPositions[i].Pair = helpers.NormalizeContractName(openPositions[i].Pair)
	}
	return openPositions, nil
}
