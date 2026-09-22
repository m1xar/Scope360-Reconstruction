package reconstructor

import (
	"sort"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/connector/binance/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/connector/binance/models"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/service/reconstructor/workers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

const (
	defaultCandleWorkers   = 4
	defaultPositionWorkers = 8
	defaultSymbolWorkers   = 4
)

func ReconstructPositions(
	groups [][]models.Trade,
	ordersByID map[int64]models.Order,
	ordersBySymbol map[string][]models.Order,
	ledger *helpers.Ledger,
	symbolCfg map[string]models.SymbolConfig,
	candleRequests chan<- helpers.CandleRequest,
	out chan<- envelope.PositionEnvelope,
) {
	type pendingCandle struct {
		env     envelope.PositionEnvelope
		replyCh chan helpers.CandleResponse
	}

	episodes := helpers.BuildEpisodesFromGroups(groups)
	pending := make([]pendingCandle, 0, len(episodes))

	for _, ep := range episodes {
		if !ep.Closed {
			continue
		}

		env := buildEnvelope(ep, ordersByID, ordersBySymbol, ledger, symbolCfg)

		if candleRequests == nil {
			out <- env
			continue
		}

		replyCh := make(chan helpers.CandleResponse, 1)
		candleRequests <- helpers.CandleRequest{
			Symbol:   ep.Symbol,
			Interval: "1m",
			StartMs:  ep.OpenAt.UnixMilli(),
			EndMs:    ep.CloseAt.UnixMilli(),
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

func buildEnvelope(
	ep helpers.Episode,
	ordersByID map[int64]models.Order,
	ordersBySymbol map[string][]models.Order,
	ledger *helpers.Ledger,
	symbolCfg map[string]models.SymbolConfig,
) envelope.PositionEnvelope {
	leverage, isolated := helpers.LeverageFor(symbolCfg, ep.Symbol)
	tp, sl := helpers.TPSLForEpisode(ep, ordersBySymbol[ep.Symbol])

	var funding, liquidationFee float64
	if ledger != nil {
		openMs, closeMs := ep.OpenAt.UnixMilli(), ep.CloseAt.UnixMilli()
		funding = ledger.FundingForRange(ep.Symbol, openMs, closeMs)
		liquidationFee = ledger.InsuranceForRange(ep.Symbol, openMs, closeMs)
	}

	return envelope.PositionEnvelope{
		Symbol:         ep.Symbol,
		Side:           ep.Side(),
		Parts:          ep.Parts,
		Orders:         helpers.OrdersForEpisode(ep, ordersByID),
		OpenAt:         ep.OpenAt,
		CloseAt:        ep.CloseAt,
		PeakSize:       ep.PeakSize,
		OpenSign:       ep.OpenSign,
		Closed:         ep.Closed,
		Leverage:       leverage,
		Isolated:       isolated,
		StopLoss:       sl,
		TakeProfit:     tp,
		Funding:        funding,
		LiquidationFee: liquidationFee,
	}
}

func LoadLedger(client *resty.Client, startMs int64) (*helpers.Ledger, error) {
	rows, err := executors.FetchAllIncome(client, startMs, 0, "")
	if err != nil {
		return nil, err
	}
	return helpers.BuildLedger(rows), nil
}

func forEachSymbol[T any](symbols []string, fn func(symbol string) ([]T, error)) ([]T, error) {
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		all      []T
		sem      = make(chan struct{}, defaultSymbolWorkers)
	)

	for _, symbol := range symbols {
		wg.Add(1)
		go func(symbol string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			rows, err := fn(symbol)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			all = append(all, rows...)
		}(symbol)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	return all, nil
}

// symbolWalk is the result of walking one symbol's trades newest first.
type symbolWalk struct {
	groups    [][]models.Trade // closed episodes, each oldest first
	openFills []models.Trade   // fills of the still-open positions
}

// walkSymbolFills walks a symbol's trades backwards in 7-day windows. With a
// cutoff it stops once the window is past it and no episode is left half
// walked, so positions that straddle the cutoff are still completed. With
// untilResolved it stops as soon as every open position seeded from
// openPositions has been walked back to its opening fill. Without either it
// walks to the retention floor.
func walkSymbolFills(
	fetch func(startMs, endMs int64) ([]models.Trade, error),
	symbol string,
	openPositions []models.PositionRisk,
	cutoff *time.Time,
	untilResolved bool,
) (symbolWalk, error) {
	var own []models.PositionRisk
	for _, p := range openPositions {
		if p.Symbol == symbol {
			own = append(own, p)
		}
	}
	segmenter := helpers.NewFillSegmenter(own)

	cutoffMs := int64(0)
	if cutoff != nil {
		cutoffMs = cutoff.UnixMilli()
	}

	var walk symbolWalk
	now := time.Now().UnixMilli()
	for _, span := range window.Backward(now, now-window.Retention.Milliseconds(), executors.TradesWindowMax.Milliseconds()) {
		if untilResolved && segmenter.Resolved() {
			break
		}
		if cutoff != nil && span.EndMs < cutoffMs && segmenter.Flat() {
			break
		}

		fills, err := fetch(span.StartMs, span.EndMs)
		if err != nil {
			return symbolWalk{}, err
		}
		walk.groups = append(walk.groups, segmenter.PushOlderBatch(fills)...)
	}
	walk.openFills = segmenter.OpenFills()
	return walk, nil
}

func collectWalks(
	client *resty.Client,
	symbols []string,
	openPositions []models.PositionRisk,
	cutoff *time.Time,
	untilResolved bool,
) ([]symbolWalk, error) {
	return forEachSymbol(symbols, func(symbol string) ([]symbolWalk, error) {
		walk, err := walkSymbolFills(func(startMs, endMs int64) ([]models.Trade, error) {
			return executors.FetchUserTradesWindow(client, symbol, startMs, endMs)
		}, symbol, openPositions, cutoff, untilResolved)
		if err != nil {
			return nil, err
		}
		return []symbolWalk{walk}, nil
	})
}

// normalizeGroupFees converts non-stable commissions of every fill in groups
// in one pass (the converter caches klines across symbols).
func normalizeGroupFees(client *resty.Client, groups [][]models.Trade) {
	flat := groupFills(groups)
	helpers.NormalizeFees(client, flat)
	byID := make(map[int64]models.Trade, len(flat))
	for _, f := range flat {
		byID[f.ID] = f
	}
	for _, g := range groups {
		for i := range g {
			g[i] = byID[g[i].ID]
		}
	}
}

func collectOrders(client *resty.Client, fills []models.Trade) ([]models.Order, error) {
	minOrder := make(map[string]int64)
	for _, fill := range fills {
		if cur, ok := minOrder[fill.Symbol]; !ok || fill.OrderID < cur {
			minOrder[fill.Symbol] = fill.OrderID
		}
	}

	symbols := make([]string, 0, len(minOrder))
	for s := range minOrder {
		symbols = append(symbols, s)
	}
	sort.Strings(symbols)

	return forEachSymbol(symbols, func(symbol string) ([]models.Order, error) {
		return executors.FetchOrdersFrom(client, symbol, minOrder[symbol])
	})
}

func GroupsClosedAfter(groups [][]models.Trade, cutoff *time.Time) [][]models.Trade {
	if cutoff == nil {
		return groups
	}

	cutoffMs := cutoff.UnixMilli()
	kept := make([][]models.Trade, 0, len(groups))
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		if group[len(group)-1].Time < cutoffMs {
			continue
		}
		kept = append(kept, group)
	}
	return kept
}

func groupFills(groups [][]models.Trade) []models.Trade {
	var out []models.Trade
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

func unionSymbols(ledger *helpers.Ledger, positions []models.PositionRisk) []string {
	set := make(map[string]struct{})
	if ledger != nil {
		for _, s := range ledger.TradeSymbols() {
			set[s] = struct{}{}
		}
	}
	for _, p := range positions {
		set[p.Symbol] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func fetchSymbolConfigLenient(client *resty.Client) map[string]models.SymbolConfig {
	cfg, err := executors.FetchSymbolConfig(client)
	if err != nil {
		return map[string]models.SymbolConfig{}
	}
	return cfg
}

func BalanceSnapshots(
	client *resty.Client,
	ledger *helpers.Ledger,
	windowStart *time.Time,
) ([]domain.UserBalanceSnapshot, error) {
	account, err := executors.FetchAccount(client)
	if err != nil {
		return nil, err
	}

	if ledger == nil {
		startMs := int64(0)
		if windowStart != nil {
			startMs = windowStart.UnixMilli()
		}
		ledger, err = LoadLedger(client, startMs)
		if err != nil {
			return nil, err
		}
	}

	return builders.BuildBalanceSnapshots(executors.StableWalletBalance(account), ledger, windowStart), nil
}

func ReconstructClosedPositions(
	client *resty.Client,
	cutoff *time.Time,
) ([]domain.Position, error) {
	openPositions, err := executors.FetchOpenPositions(client)
	if err != nil {
		return nil, err
	}

	// Any position closed after the cutoff left REALIZED_PNL/COMMISSION
	// income after it, so income from the cutoff is enough to find the
	// symbols to walk. cutoff == nil keeps the full retention.
	ledgerStartMs := int64(0)
	if cutoff != nil {
		ledgerStartMs = cutoff.UnixMilli()
	}
	ledger, err := LoadLedger(client, ledgerStartMs)
	if err != nil {
		return nil, err
	}

	symbols := unionSymbols(ledger, openPositions)
	if len(symbols) == 0 {
		return []domain.Position{}, nil
	}

	walks, err := collectWalks(client, symbols, openPositions, cutoff, false)
	if err != nil {
		return nil, err
	}
	var groups [][]models.Trade
	for _, w := range walks {
		groups = append(groups, w.groups...)
	}
	groups = GroupsClosedAfter(groups, cutoff)
	if len(groups) == 0 {
		return []domain.Position{}, nil
	}
	normalizeGroupFees(client, groups)

	// A surviving position may have opened before the cutoff: extend the
	// ledger back to its open for funding, insurance fees and balance.
	if cutoff != nil {
		earliestMs := groups[0][0].Time
		for _, g := range groups {
			if g[0].Time < earliestMs {
				earliestMs = g[0].Time
			}
		}
		if earliestMs < ledgerStartMs {
			earlier, err := executors.FetchAllIncome(client, earliestMs, ledgerStartMs-1, "")
			if err != nil {
				return nil, err
			}
			ledger = helpers.BuildLedger(append(earlier, ledger.Entries...))
		}
	}

	symbolCfg := fetchSymbolConfigLenient(client)

	orders, err := collectOrders(client, groupFills(groups))
	if err != nil {
		return nil, err
	}

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, candleRequests, defaultCandleWorkers)

	envelopes := make(chan envelope.PositionEnvelope)
	positionsCh := make(chan domain.Position)

	go func() {
		ReconstructPositions(
			groups,
			helpers.IndexOrdersByID(orders),
			helpers.GroupOrdersBySymbol(orders),
			ledger,
			symbolCfg,
			candleRequests,
			envelopes,
		)
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

	if snapshots, err := BalanceSnapshots(client, ledger, helpers.BalanceWindowStart(positions, cutoff)); err == nil {
		helpers.AttachBalanceInit(&positions, snapshots)
	}

	return positions, nil
}

func ReconstructOpenPositions(client *resty.Client) ([]domain.OpenPosition, error) {
	raw, err := executors.FetchOpenPositions(client)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return []domain.OpenPosition{}, nil
	}

	symbolCfg := fetchSymbolConfigLenient(client)

	walks, err := collectWalks(client, unionSymbols(nil, raw), raw, nil, true)
	if err != nil {
		return nil, err
	}
	var openFills []models.Trade
	for _, w := range walks {
		openFills = append(openFills, w.openFills...)
	}
	helpers.NormalizeFees(client, openFills)

	var orders []models.Order
	if len(openFills) > 0 {
		orders, err = collectOrders(client, openFills)
		if err != nil {
			return nil, err
		}
	}

	return builders.BuildOpenPositions(raw, openFills, helpers.IndexOrdersByID(orders), symbolCfg), nil
}
