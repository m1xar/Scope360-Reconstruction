package reconstructor

import (
	"math"
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
		floor     = time.Now().Add(-maxFillLookback)
		seen      = make(map[string]struct{})
		cursor    string
	)

	for {
		page, err := executors.FetchFills(client, cursor)
		if err != nil {
			return FillWalk{}, err
		}
		if len(page) == 0 {
			break
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

		if len(fresh) > 0 {
			pages = append(pages, fresh)
			walk.Groups = append(walk.Groups, segmenter.PushOlderBatch(fresh)...)
		}

		if oldest.IsZero() || len(fresh) == 0 || len(page) < executors.FillsPageSize {
			break
		}
		if oldest.Before(floor) {
			break
		}
		if cutoff != nil && oldest.Before(*cutoff) && segmenter.Flat() {
			break
		}
		cursor = executors.FormatKrakenTime(oldest)
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

	// Fills before a position's OpenTime are discarded below, so the fetch
	// only needs to reach back to the oldest open position.
	fills, err := executors.FetchAllFills(client, daysCovering(positions))
	if err != nil {
		return
	}

	posIdx := 0
	for _, rawPos := range raw {
		if rawPos.Size.Float64() <= 0 {
			continue
		}
		if posIdx >= len(positions) {
			return
		}

		openTime := positions[posIdx].OpenTime
		symbol := strings.ToUpper(rawPos.Symbol)
		matched := make([]models.Fill, 0)
		for _, fill := range fills {
			if strings.ToUpper(fill.Symbol) != symbol {
				continue
			}
			fillTime, err := helpers.ParseTime(fill.FillTime)
			if err != nil || fillTime.Before(openTime) {
				continue
			}
			matched = append(matched, fill)
		}

		positions[posIdx].Orders = builders.BuildOpenOrdersFromFills(matched, positions[posIdx].ID)
		posIdx++
	}
}

// daysCovering returns the days argument for FetchAllFills that reaches the
// oldest OpenTime among positions; 0 (full history) when none is known.
func daysCovering(positions []domain.OpenPosition) int {
	var oldest time.Time
	for _, pos := range positions {
		if pos.OpenTime.IsZero() {
			return 0
		}
		if oldest.IsZero() || pos.OpenTime.Before(oldest) {
			oldest = pos.OpenTime
		}
	}
	if oldest.IsZero() {
		return 0
	}
	return int(math.Ceil(time.Since(oldest).Hours()/24)) + 1
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
