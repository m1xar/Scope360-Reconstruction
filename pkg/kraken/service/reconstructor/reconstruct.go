package reconstructor

import (
	"sort"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/workers"
)

const maxFillLookback = 3 * 365 * 24 * time.Hour

const defaultCandleWorkers = 4

type FillWalk struct {
	Groups [][]models.Fill
	Fills  []models.Fill
}

func (w FillWalk) EarliestOpen() time.Time {
	var earliest time.Time
	for _, group := range w.Groups {
		if len(group) == 0 {
			continue
		}
		at, err := helpers.ParseTime(group[0].FillTime)
		if err != nil {
			continue
		}
		if earliest.IsZero() || at.Before(earliest) {
			earliest = at
		}
	}
	return earliest
}

func CollectClosedEpisodes(
	client *resty.Client,
	cutoff *time.Time,
) (FillWalk, error) {
	openPositions, err := executors.FetchOpenPositions(client)
	if err != nil {
		return FillWalk{}, err
	}

	var (
		segmenter = helpers.NewFillSegmenter(openPositions)
		walk      FillWalk
		pages     [][]models.Fill
	)

	err = walkFillsBack(client, func(fresh []models.Fill, oldest time.Time) bool {
		pages = append(pages, fresh)
		walk.Groups = append(walk.Groups, segmenter.PushOlderBatch(fresh)...)
		return cutoff != nil && oldest.Before(*cutoff) && segmenter.Flat()
	})
	if err != nil {
		return FillWalk{}, err
	}

	for i := len(pages) - 1; i >= 0; i-- {
		walk.Fills = append(walk.Fills, pages[i]...)
	}
	sort.SliceStable(walk.Fills, func(i, j int) bool {
		ti, _ := helpers.ParseTime(walk.Fills[i].FillTime)
		tj, _ := helpers.ParseTime(walk.Fills[j].FillTime)
		if ti.Equal(tj) {
			return walk.Fills[i].FillID < walk.Fills[j].FillID
		}
		return ti.Before(tj)
	})
	return walk, nil
}

// walkFillsBack pages the fill history newest first, handing each page's
// unseen fills and its oldest fill time to visit, until visit asks to stop,
// the history is exhausted or the lookback floor is reached.
func walkFillsBack(client *resty.Client, visit func(fresh []models.Fill, oldest time.Time) (stop bool)) error {
	var (
		floor  = time.Now().Add(-maxFillLookback)
		seen   = make(map[string]struct{})
		cursor string
	)
	for {
		page, err := executors.FetchFills(client, cursor)
		if err != nil {
			return err
		}
		if len(page) == 0 {
			return nil
		}

		fresh := make([]models.Fill, 0, len(page))
		oldest := time.Time{}
		for _, fill := range page {
			at, err := helpers.ParseTime(fill.FillTime)
			if err != nil {
				continue
			}
			if oldest.IsZero() || at.Before(oldest) {
				oldest = at
			}
			key := builderFillKey(fill)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			fresh = append(fresh, fill)
		}
		if oldest.IsZero() || len(fresh) == 0 {
			return nil
		}
		if visit(fresh, oldest) || len(page) < executors.FillsPageSize || oldest.Before(floor) {
			return nil
		}
		cursor = executors.FormatKrakenTime(oldest)
	}
}

func builderFillKey(fill models.Fill) string {
	if fill.FillID != "" {
		return fill.FillID
	}
	return fill.Symbol + "|" + fill.OrderID + "|" + fill.FillTime
}

func GroupsClosedAfter(groups [][]models.Fill, cutoff *time.Time) [][]models.Fill {
	if cutoff == nil {
		return groups
	}

	kept := make([][]models.Fill, 0, len(groups))
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		at, err := helpers.ParseTime(group[len(group)-1].FillTime)
		if err != nil || at.Before(*cutoff) {
			continue
		}
		kept = append(kept, group)
	}
	return kept
}

func EnrichMAEMFE(client *resty.Client, positions *[]domain.Position, symbolByPair map[string]string) {
	if positions == nil || len(*positions) == 0 {
		return
	}

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, candleRequests, defaultCandleWorkers)

	type pendingCandle struct {
		idx     int
		replyCh chan helpers.CandleResponse
	}
	pending := make([]pendingCandle, 0, len(*positions))

	for i := range *positions {
		pos := &(*positions)[i]
		if pos.ClosedAt == nil {
			continue
		}

		replyCh := make(chan helpers.CandleResponse, 1)
		symbol := symbolByPair[pos.Pair]
		if symbol == "" {
			symbol = helpers.SymbolFromPair(pos.Pair)
		}

		candleRequests <- helpers.CandleRequest{
			TickType: "trade",
			Symbol:   symbol,
			Interval: "1m",
			StartMs:  pos.CreatedAt.UnixMilli(),
			EndMs:    pos.ClosedAt.UnixMilli(),
			ReplyCh:  replyCh,
		}
		pending = append(pending, pendingCandle{idx: i, replyCh: replyCh})
	}
	close(candleRequests)

	for _, p := range pending {
		resp := <-p.replyCh
		if resp.Err != nil {
			continue
		}
		high, low := helpers.GetHighLow(resp.Candles)
		helpers.ApplyMAEMFE(&(*positions)[p.idx], high, low)
	}
}

func BuildPairMap(client *resty.Client, symbols []string) map[string]string {
	out := make(map[string]string, len(symbols))
	for _, symbol := range symbols {
		sym := strings.ToUpper(strings.TrimSpace(symbol))
		if sym == "" {
			continue
		}
		if _, ok := out[sym]; ok {
			continue
		}
		ticker, err := executors.FetchTicker(client, sym)
		if err == nil && ticker.Pair != "" {
			out[sym] = helpers.NormalizePairText(ticker.Pair)
			continue
		}
		out[sym] = helpers.NormalizePairFallback(sym)
	}
	return out
}

func EnrichOpenPositionOrders(
	client *resty.Client,
	raw []models.OpenPosition,
	positions []domain.OpenPosition,
) {
	if len(raw) == 0 || len(positions) == 0 {
		return
	}

	// Kraken reports the epoch as fillTime for open positions and its fills
	// omit liquidations, so each position's orders come from its position
	// events, walked newest first down to the event that opened it. Fills
	// are the fallback when the events are unavailable.
	var openFills map[string][]models.Fill
	posIdx := 0
	for _, rawPos := range raw {
		if rawPos.Size.Float64() <= 0 {
			continue
		}
		if posIdx >= len(positions) {
			return
		}
		pos := &positions[posIdx]
		posIdx++

		symbol := strings.ToUpper(strings.TrimSpace(rawPos.Symbol))
		if events, err := collectOpenEvents(client, symbol); err == nil && len(events) > 0 {
			pos.Orders = builders.BuildOpenOrdersFromEvents(events, pos.ID)
			if pos.OpenTime.Unix() <= 0 {
				pos.OpenTime = builders.EventTime(events[0])
			}
			continue
		}

		if openFills == nil {
			fills, err := collectOpenFills(client, raw)
			if err != nil {
				return
			}
			openFills = fills
		}
		pos.Orders = builders.BuildOpenOrdersFromFills(openFills[symbol], pos.ID)
	}
}

// openEventsMaxPages caps the walk back through a symbol's position events
// (1000 per page; funding adds one event per hour while a position is open,
// so 30 pages cover well over three years).
const openEventsMaxPages = 30

// collectOpenEvents returns the execution events of the current position on
// symbol, oldest first, starting with the event that opened it (from flat
// or by flipping through zero). Nil when no opening event was found within
// openEventsMaxPages.
func collectOpenEvents(client *resty.Client, symbol string) ([]models.PositionUpdate, error) {
	var collected []models.PositionUpdate
	continuation := ""
	for page := 0; page < openEventsMaxPages; page++ {
		resp, err := executors.FetchPositionEventsPageDesc(client, symbol, continuation)
		if err != nil {
			return nil, err
		}
		for _, ev := range resp.Elements {
			upd := ev.Event.PositionUpdate
			if upd.Timestamp == 0 {
				upd.Timestamp = ev.Timestamp
			}
			if upd.NewPosition.Float64() != upd.OldPosition.Float64() {
				collected = append(collected, upd)
			}
			if builders.IsOpeningEvent(upd) {
				for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
					collected[i], collected[j] = collected[j], collected[i]
				}
				return collected, nil
			}
		}
		if resp.ContinuationToken == "" || len(resp.Elements) == 0 {
			break
		}
		continuation = resp.ContinuationToken
	}
	return nil, nil
}

// collectOpenFills pages fills newest first until the walker seeded with the
// open positions is resolved (or the lookback floor is hit) and returns the
// fills of each open position, oldest first, keyed by symbol.
func collectOpenFills(client *resty.Client, openPositions []models.OpenPosition) (map[string][]models.Fill, error) {
	segmenter := helpers.NewFillSegmenter(openPositions)
	if segmenter.Resolved() {
		return map[string][]models.Fill{}, nil
	}
	err := walkFillsBack(client, func(fresh []models.Fill, _ time.Time) bool {
		segmenter.PushOlderBatch(fresh)
		return segmenter.Resolved()
	})
	if err != nil {
		return nil, err
	}
	return segmenter.OpenFills(), nil
}

func ReconstructClosedPositions(
	client *resty.Client,
	cutoff *time.Time,
) ([]domain.Position, error) {
	walk, err := CollectClosedEpisodes(client, cutoff)
	if err != nil {
		return nil, err
	}

	groups := GroupsClosedAfter(walk.Groups, cutoff)
	if len(groups) == 0 {
		return []domain.Position{}, nil
	}

	since := walk.EarliestOpen()

	positionEvents, err := executors.FetchAllPositionEventsSince(client, since)
	if err != nil {
		return nil, err
	}

	pairBySymbol := BuildPairMap(client, helpers.SymbolsFromFillsAndEvents(walk.Fills, positionEvents))
	positions, err := builders.BuildClosedPositions(groups, positionEvents, pairBySymbol)
	if err != nil {
		return nil, err
	}

	EnrichMAEMFE(client, &positions, helpers.RawSymbolByPair(helpers.SymbolsFromFillsAndEvents(walk.Fills, positionEvents), pairBySymbol))

	if cutoff != nil {
		filtered := positions[:0]
		for _, pos := range positions {
			if pos.ClosedAt != nil && !pos.ClosedAt.Before(*cutoff) {
				filtered = append(filtered, pos)
			}
		}
		positions = filtered
	}

	if logs, err := executors.FetchAllAccountLogSince(client, since); err == nil {
		snapshots := builders.BuildBalanceSnapshots(logs)
		helpers.AttachBalanceInit(&positions, snapshots)
	}

	return positions, nil
}
