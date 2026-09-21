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

func CollectFills(client *resty.Client, symbols []string) ([]models.Trade, error) {
	fills, err := forEachSymbol(symbols, func(symbol string) ([]models.Trade, error) {
		return executors.FetchAllUserTrades(client, symbol)
	})
	if err != nil {
		return nil, err
	}

	sort.SliceStable(fills, func(i, j int) bool {
		if fills[i].Time == fills[j].Time {
			return fills[i].ID < fills[j].ID
		}
		return fills[i].Time < fills[j].Time
	})

	helpers.NormalizeFees(client, fills)
	return fills, nil
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

func SegmentFills(fills []models.Trade, openPositions []models.PositionRisk) [][]models.Trade {
	segmenter := helpers.NewFillSegmenter(openPositions)
	return segmenter.PushOlderBatch(fills)
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

	ledger, err := LoadLedger(client, 0)
	if err != nil {
		return nil, err
	}

	symbols := unionSymbols(ledger, openPositions)
	if len(symbols) == 0 {
		return []domain.Position{}, nil
	}

	fills, err := CollectFills(client, symbols)
	if err != nil {
		return nil, err
	}

	groups := GroupsClosedAfter(SegmentFills(fills, openPositions), cutoff)
	if len(groups) == 0 {
		return []domain.Position{}, nil
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

	fills, err := CollectFills(client, unionSymbols(nil, raw))
	if err != nil {
		return nil, err
	}

	var openFills []models.Trade
	for _, ep := range helpers.OpenEpisodes(fills) {
		for _, part := range ep.Parts {
			openFills = append(openFills, part.Fill)
		}
	}

	var orders []models.Order
	if len(openFills) > 0 {
		orders, err = collectOrders(client, openFills)
		if err != nil {
			return nil, err
		}
	}

	return builders.BuildOpenPositions(raw, fills, helpers.IndexOrdersByID(orders), symbolCfg), nil
}
