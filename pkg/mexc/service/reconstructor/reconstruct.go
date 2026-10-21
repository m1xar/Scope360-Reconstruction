package reconstructor

import (
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

const (
	defaultCandleWorkers   = 4
	defaultPositionWorkers = 8
)

const fundingLookback = 10 * time.Minute

type orderWalk struct {
	groups    [][]models.Order
	openFills map[string][]models.Order
}

func (w orderWalk) earliestOpenMs() int64 {
	earliest := int64(0)
	for _, group := range w.groups {
		if len(group) == 0 {
			continue
		}
		ts := group[0].UpdateTime
		if earliest == 0 || ts < earliest {
			earliest = ts
		}
	}
	return earliest
}

func walkOrders(
	fetch func(startMs, endMs int64) ([]models.Order, error),
	openPositions []models.OpenPosition,
	contractSizes map[string]float64,
	cutoff *time.Time,
	untilResolved bool,
) (orderWalk, error) {
	segmenter := helpers.NewFillSegmenter(openPositions, contractSizes)
	cutoffMs := window.StartMs(cutoff)

	var walk orderWalk
	now := time.Now().UnixMilli()
	floorMs := now - window.Retention.Milliseconds()
	for _, span := range window.Backward(now, floorMs, executors.OrdersWindowMax.Milliseconds()) {
		settled := segmenter.Flat()
		if untilResolved {
			settled = segmenter.Resolved()
		}
		if cutoff == nil && untilResolved && settled {
			break
		}
		if cutoff != nil && span.EndMs < cutoffMs && settled {
			break
		}

		orders, err := fetch(span.StartMs, span.EndMs)
		if err != nil {
			return orderWalk{}, err
		}
		walk.groups = append(walk.groups, segmenter.PushOlderBatch(orders)...)
	}
	walk.openFills = segmenter.OpenFills()
	return walk, nil
}

func GroupsClosedAfter(groups [][]models.Order, cutoff *time.Time) [][]models.Order {
	if cutoff == nil {
		return groups
	}

	cutoffMs := cutoff.UnixMilli()
	kept := make([][]models.Order, 0, len(groups))
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		if group[len(group)-1].UpdateTime < cutoffMs {
			continue
		}
		kept = append(kept, group)
	}
	return kept
}

func ReconstructPositions(
	groups [][]models.Order,
	funding []models.FundingRecord,
	contractSizes map[string]float64,
	candleRequests chan<- helpers.CandleRequest,
	out chan<- envelope.PositionEnvelope,
) {
	type pendingCandle struct {
		env     envelope.PositionEnvelope
		replyCh chan helpers.CandleResponse
	}

	episodes := helpers.BuildEpisodesFromGroups(groups, contractSizes)
	pending := make([]pendingCandle, 0, len(episodes))

	for _, ep := range episodes {
		if !ep.Closed {
			continue
		}

		env := buildEnvelope(ep, funding, contractSizes)

		if candleRequests == nil {
			out <- env
			continue
		}

		replyCh := make(chan helpers.CandleResponse, 1)
		candleRequests <- helpers.CandleRequest{
			Symbol:  ep.Symbol,
			Bar:     "1m",
			StartMs: ep.OpenAt.UnixMilli(),
			EndMs:   ep.CloseAt.UnixMilli(),
			ReplyCh: replyCh,
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

func buildEnvelope(ep helpers.Episode, funding []models.FundingRecord, contractSizes map[string]float64) envelope.PositionEnvelope {
	ctx := helpers.ContextForEpisode(ep)
	return envelope.PositionEnvelope{
		Symbol:        ep.Symbol,
		Side:          ep.Side(),
		ContractSizes: contractSizes,
		Parts:         ep.Parts,
		OpenAt:        ep.OpenAt,
		CloseAt:       ep.CloseAt,
		OpenSign:      ep.OpenSign,
		Closed:        ep.Closed,
		Leverage:      ctx.Leverage,
		Isolated:      ctx.Isolated,
		StopLoss:      ctx.StopLoss,
		TakeProfit:    ctx.TakeProfit,
		Funding:       helpers.FundingForRange(funding, ep.Symbol, ep.PositionType, ep.OpenAt.UnixMilli(), ep.CloseAt.UnixMilli()),
	}
}

func fetchContractSizes(client *resty.Client) map[string]float64 {
	sizes := make(map[string]float64)

	details, err := executors.FetchAllContractDetails(client)
	if err != nil {
		return sizes
	}
	for _, detail := range details {
		if detail.Symbol != "" && detail.ContractSize > 0 {
			sizes[detail.Symbol] = detail.ContractSize
		}
	}
	return sizes
}

func fillMissingContractSizes(client *resty.Client, sizes map[string]float64, symbols []string) {
	for _, symbol := range symbols {
		if symbol == "" || sizes[symbol] > 0 {
			continue
		}
		detail, err := executors.FetchContractDetail(client, symbol)
		if err != nil || detail.ContractSize <= 0 {
			continue
		}
		sizes[symbol] = detail.ContractSize
	}
}

func walkSymbols(walk orderWalk, open []models.OpenPosition) []string {
	set := make(map[string]struct{})
	for _, group := range walk.groups {
		for _, order := range group {
			set[order.Symbol] = struct{}{}
		}
	}
	for _, fills := range walk.openFills {
		for _, order := range fills {
			set[order.Symbol] = struct{}{}
		}
	}
	for _, pos := range open {
		set[pos.Symbol] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for symbol := range set {
		out = append(out, symbol)
	}
	sort.Strings(out)
	return out
}

func FetchStableEquity(client *resty.Client) (float64, error) {
	assets, err := executors.FetchAssets(client)
	if err == nil {
		var total float64
		var found bool
		for _, asset := range assets {
			if helpers.IsStableCurrency(asset.Currency) {
				total += asset.Equity
				found = true
			}
		}
		if found {
			return helpers.Round8(total), nil
		}
	}

	asset, fallbackErr := executors.FetchUSDTAsset(client)
	if fallbackErr != nil {
		if err != nil {
			return 0, err
		}
		return 0, fallbackErr
	}
	return helpers.Round8(asset.Equity), nil
}

func ReconstructClosedPositions(client *resty.Client, cutoff *time.Time) ([]domain.Position, error) {
	d, err := Load(client, cutoff, scope.Closed)
	if err != nil {
		return nil, err
	}
	return d.ClosedPositions()
}
