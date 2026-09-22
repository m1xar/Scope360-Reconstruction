package reconstructor

import (
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
)

func ReconstructPositions(
	groups [][]models.Fill,
	ordersByID map[string]models.Order,
	fundings []models.FundingFee,
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

		env := buildEnvelope(ep, ordersByID, fundings, instruments)

		if candleRequests == nil {
			out <- env
			continue
		}

		replyCh := make(chan helpers.CandleResponse, 1)
		candleRequests <- helpers.CandleRequest{
			InstID:  ep.InstID,
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
	fundings []models.FundingFee,
	instruments map[string]models.Instrument,
) envelope.PositionEnvelope {
	orders := helpers.OrdersForEpisode(ep, ordersByID)
	leverage, isolated, tp, sl := helpers.OrderContext(orders)

	return envelope.PositionEnvelope{
		InstID:     ep.InstID,
		Side:       ep.Side(),
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
		Funding:    helpers.FundingForRange(fundings, ep.InstID, ep.OpenAt.UnixMilli(), ep.CloseAt.UnixMilli()),
	}
}

const maxFillLookback = 3 * 365 * 24 * time.Hour

const historyLookback = 1 * time.Minute

const (
	defaultCandleWorkers   = 4
	defaultPositionWorkers = 8
)

type FillWalk struct {
	Groups [][]models.Fill
	Fills  []models.Fill
}

func (w FillWalk) EarliestOpenMs() int64 {
	earliest := int64(0)
	for _, group := range w.Groups {
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
	instruments map[string]models.Instrument,
	openPositions []models.OpenPosition,
	cutoff *time.Time,
	reachMs int64,
) (FillWalk, error) {
	var (
		segmenter = helpers.NewFillSegmenter(openPositions, instruments)
		walk      FillWalk
		pages     [][]models.Fill
		floorMs   = time.Now().Add(-maxFillLookback).UnixMilli()
		seen      = make(map[string]struct{})
		after     string
	)

	for {
		page, err := executors.FetchFillsPage(client, baseURL, after, executors.FillsPageLimit)
		if err != nil {
			return FillWalk{}, err
		}
		if len(page) == 0 {
			break
		}

		fresh := make([]models.Fill, 0, len(page))
		oldestMs := int64(0)
		for _, fill := range page {
			ts := helpers.MustInt64(fill.Ts)
			if ts > 0 && (oldestMs == 0 || ts < oldestMs) {
				oldestMs = ts
			}
			if _, ok := seen[fill.TradeID]; ok {
				continue
			}
			seen[fill.TradeID] = struct{}{}
			fresh = append(fresh, fill)
		}

		if len(fresh) > 0 {
			pages = append(pages, fresh)
			walk.Groups = append(walk.Groups, segmenter.PushOlderBatch(fresh)...)
		}

		if len(fresh) == 0 || oldestMs == 0 || len(page) < executors.FillsPageLimit {
			break
		}
		if oldestMs < floorMs {
			break
		}
		if cutoff != nil && oldestMs < cutoff.UnixMilli() && (reachMs == 0 || oldestMs < reachMs) && segmenter.Flat() {
			break
		}
		after = page[len(page)-1].TradeID
	}

	for i := len(pages) - 1; i >= 0; i-- {
		walk.Fills = append(walk.Fills, pages[i]...)
	}
	sort.SliceStable(walk.Fills, func(i, j int) bool {
		ti, tj := helpers.MustInt64(walk.Fills[i].Ts), helpers.MustInt64(walk.Fills[j].Ts)
		if ti == tj {
			return walk.Fills[i].TradeID < walk.Fills[j].TradeID
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
	cutoff *time.Time,
) ([]domain.Position, error) {
	d, err := Load(client, baseURL, cutoff, scope.Closed)
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
