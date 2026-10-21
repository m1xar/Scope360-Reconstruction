package reconstructor

import (
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
)

const (
	defaultCandleWorkers   = 4
	defaultPositionWorkers = 8
)

const maxFillLookback = 3 * 365 * 24 * time.Hour

const openPositionOrdersLookback = 7 * 24 * time.Hour

const billsLookback = 10 * time.Minute

func ReconstructPositions(
	groups [][]models.Fill,
	ordersByID map[string]models.Order,
	bills []models.Bill,
	instruments map[string]models.Instrument,
	candleRequests chan<- helpers.CandleRequest,
	out chan<- envelope.PositionEnvelope,
) {
	type pendingCandle struct {
		env     envelope.PositionEnvelope
		replyCh chan helpers.CandleResponse
	}

	episodes := helpers.BuildEpisodesFromGroups(groups, instruments)
	pending := make([]pendingCandle, 0, len(episodes))

	for _, ep := range episodes {
		if !ep.Closed {
			continue
		}

		env := buildEnvelope(ep, ordersByID, bills, instruments)

		if candleRequests == nil {
			out <- env
			continue
		}

		replyCh := make(chan helpers.CandleResponse, 1)
		candleRequests <- helpers.CandleRequest{
			InstId:  ep.InstID,
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

func buildEnvelope(
	ep helpers.Episode,
	ordersByID map[string]models.Order,
	bills []models.Bill,
	instruments map[string]models.Instrument,
) envelope.PositionEnvelope {
	orders := helpers.OrdersForParts(ep.Parts, ordersByID)
	leverage, isolated := helpers.OrderContext(orders)
	side := ep.Side()
	tp, sl := helpers.ProtectiveLevels(orders, side)

	return envelope.PositionEnvelope{
		InstID:     ep.InstID,
		Side:       side,
		Instrument: helpers.InstrumentFor(instruments, ep.InstID),
		Parts:      ep.Parts,
		Orders:     orders,
		OpenAt:     ep.OpenAt,
		CloseAt:    ep.CloseAt,
		PeakSize:   ep.PeakSize,
		OpenSign:   ep.OpenSign,
		Closed:     ep.Closed,
		Leverage:   leverage,
		Isolated:   isolated,
		StopLoss:   sl,
		TakeProfit: tp,
		Funding:    helpers.FundingForRange(bills, ep.InstID, ep.OpenAt.UnixMilli(), ep.CloseAt.UnixMilli()),
	}
}

type FillWalk struct {
	Groups    [][]models.Fill
	Fills     []models.Fill
	OpenFills map[string][]models.Fill
}

func earliestOpenMs(groups [][]models.Fill) int64 {
	earliest := int64(0)
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		ts := helpers.MustInt64(group[0].Ts)
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
	client *resty.Client,
	baseURL string,
	openPositions []models.OpenPosition,
	cutoff *time.Time,
	reachMs int64,
) (FillWalk, error) {
	var (
		segmenter   = helpers.NewFillSegmenter(openPositions)
		walk        FillWalk
		pages       [][]models.Fill
		now         = time.Now().UnixMilli()
		floorMs     = now - maxFillLookback.Milliseconds()
		retentionMs = now - executors.FillsRetention.Milliseconds()
		seen        = make(map[string]struct{})
	)

	for windowEnd := now; windowEnd > floorMs; {
		windowBegin := windowEnd - executors.WindowSize.Milliseconds()
		if windowBegin < floorMs {
			windowBegin = floorMs
		}

		page, err := executors.FetchSwapAndFuturesFillsWindow(client, baseURL, windowBegin, windowEnd)
		if err != nil {
			return FillWalk{}, err
		}

		fresh := make([]models.Fill, 0, len(page))
		for _, fill := range page {
			key := fill.BillId
			if key == "" {
				key = fill.TradeId + "|" + fill.OrdId + "|" + fill.Ts
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			fresh = append(fresh, fill)
		}
		if len(fresh) > 0 {
			pages = append(pages, fresh)
			walk.Groups = append(walk.Groups, segmenter.PushOlderBatch(fresh)...)
		}
		windowEnd = windowBegin

		if len(page) == 0 && windowBegin < retentionMs {
			break
		}
		reached := segmenter.Resolved() || reachMs == 0 || windowBegin < reachMs
		if cutoff != nil && windowBegin < cutoff.UnixMilli() && reached && segmenter.Flat() {
			break
		}
	}

	walk.OpenFills = segmenter.OpenFills()
	for i := len(pages) - 1; i >= 0; i-- {
		walk.Fills = append(walk.Fills, pages[i]...)
	}
	sort.SliceStable(walk.Fills, func(i, j int) bool {
		ti, tj := helpers.MustInt64(walk.Fills[i].Ts), helpers.MustInt64(walk.Fills[j].Ts)
		if ti == tj {
			return helpers.MustInt64(walk.Fills[i].BillId) < helpers.MustInt64(walk.Fills[j].BillId)
		}
		return ti < tj
	})
	return walk, nil
}

func GroupsClosedAfter(groups [][]models.Fill, cutoff *time.Time) [][]models.Fill {
	if cutoff == nil {
		return groups
	}

	cutoffMs := cutoff.UnixMilli()
	kept := make([][]models.Fill, 0, len(groups))
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		if helpers.MustInt64(group[len(group)-1].Ts) < cutoffMs {
			continue
		}
		kept = append(kept, group)
	}
	return kept
}

func ReconstructClosedPositions(
	client *resty.Client,
	baseURL string,
	days int,
) ([]domain.Position, error) {
	d, err := Load(client, baseURL, helpers.CutoffFromDays(days), scope.Closed)
	if err != nil {
		return nil, err
	}
	return d.ClosedPositions()
}

func ReconstructOpenPositions(
	client *resty.Client,
	baseURL string,
) ([]domain.OpenPosition, error) {
	d, err := Load(client, baseURL, nil, scope.Open)
	if err != nil {
		return nil, err
	}
	return d.OpenPositions()
}
