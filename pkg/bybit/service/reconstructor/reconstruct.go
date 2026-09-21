package reconstructor

import (
	"sort"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit/models"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor/workers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

const (
	defaultCandleWorkers   = 8
	defaultPositionWorkers = 8
	weekChunk              = 4
)

type Walk struct {
	Entries  []models.LedgerEntry
	Orders   []models.Order
	Closed   []models.ClosedPnl
	Fills    []helpers.Fill
	Groups   [][]helpers.Fill
	OldestMs int64
	Weeks    int
}

type weekData struct {
	window  executors.Window
	entries []models.LedgerEntry
	orders  []models.Order
	closed  []models.ClosedPnl
	err     error
}

func fetchWeek(client *resty.Client, w executors.Window, withOrders bool) weekData {
	data := weekData{window: w}
	var wg sync.WaitGroup
	var mu sync.Mutex
	setErr := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		if err != nil && data.err == nil {
			data.err = err
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := executors.FetchLedgerWindow(client, w, executors.LedgerFilter{})
		data.entries = rows
		setErr(err)
	}()
	if withOrders {
		wg.Add(2)
		go func() {
			defer wg.Done()
			rows, err := executors.FetchOrdersWindow(client, w)
			data.orders = rows
			setErr(err)
		}()
		go func() {
			defer wg.Done()
			rows, err := executors.FetchClosedPnlWindow(client, w)
			data.closed = rows
			setErr(err)
		}()
	}
	wg.Wait()
	return data
}

func CollectWeeks(
	client *resty.Client,
	openPositions []models.Position,
	cutoff *time.Time,
	untilResolved bool,
) (Walk, error) {
	var walk Walk
	windows := executors.Windows(0, time.Now().UnixMilli())
	segmenter := helpers.NewFillSegmenter(openPositions)
	hedge := helpers.HedgeSymbols(nil, openPositions)
	ordersByID := make(map[string]models.Order)
	seenEntries := make(map[string]struct{})
	seenOrders := make(map[string]struct{})

	cutoffMs := int64(0)
	if cutoff != nil {
		cutoffMs = cutoff.UnixMilli()
	}

	for i := 0; i < len(windows); i += weekChunk {
		batch := windows[i:min(i+weekChunk, len(windows))]
		results := make([]weekData, len(batch))

		var wg sync.WaitGroup
		for j, w := range batch {
			wg.Add(1)
			go func(j int, w executors.Window) {
				defer wg.Done()
				results[j] = fetchWeek(client, w, true)
			}(j, w)
		}
		wg.Wait()

		stop := false
		for _, week := range results {
			if week.err != nil {
				return Walk{}, week.err
			}
			walk.Weeks++
			walk.OldestMs = week.window.StartMs

			for _, o := range week.orders {
				if _, ok := seenOrders[o.OrderID]; ok {
					continue
				}
				seenOrders[o.OrderID] = struct{}{}
				ordersByID[o.OrderID] = o
				walk.Orders = append(walk.Orders, o)
				if idx := int(o.PositionIdx.Int64()); idx == models.PositionIdxLong || idx == models.PositionIdxShort {
					hedge[o.Symbol] = true
				}
			}
			walk.Closed = append(walk.Closed, week.closed...)

			fresh := make([]models.LedgerEntry, 0, len(week.entries))
			for _, row := range week.entries {
				key := ledgerKey(row)
				if _, ok := seenEntries[key]; ok {
					continue
				}
				seenEntries[key] = struct{}{}
				fresh = append(fresh, row)
			}
			offset := len(walk.Entries)
			walk.Entries = append(walk.Entries, fresh...)

			fills := helpers.BuildFills(fresh, ordersByID, hedge)
			for k := range fills {
				fills[k].Seq += offset
			}
			walk.Fills = append(walk.Fills, fills...)
			walk.Groups = append(walk.Groups, segmenter.PushOlderBatch(fills)...)

			if untilResolved && segmenter.Resolved() {
				stop = true
				break
			}
			if !untilResolved && cutoff != nil && week.window.StartMs < cutoffMs && segmenter.Flat() {
				stop = true
				break
			}
		}
		if stop {
			break
		}
	}

	walk.Fills = helpers.BuildFills(walk.Entries, ordersByID, hedge)
	helpers.NormalizeFees(client, walk.Fills)
	walk.Groups = helpers.NewFillSegmenter(openPositions).PushOlderBatch(walk.Fills)
	helpers.SortFillsAsc(walk.Fills)

	return walk, nil
}

func ledgerKey(row models.LedgerEntry) string {
	return row.ID + "|" + row.Type + "|" + row.Currency + "|" + row.TradeID + "|" + row.TransactionTime.String() + "|" + row.Change
}

func GroupsClosedAfter(groups [][]helpers.Fill, cutoff *time.Time) [][]helpers.Fill {
	if cutoff == nil {
		return groups
	}

	cutoffMs := cutoff.UnixMilli()
	kept := make([][]helpers.Fill, 0, len(groups))
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		if group[len(group)-1].TimeMs() < cutoffMs {
			continue
		}
		kept = append(kept, group)
	}
	return kept
}

func ReconstructPositions(
	groups [][]helpers.Fill,
	ordersByID map[string]models.Order,
	ordersBySymbol map[string][]models.Order,
	closedByOrder map[string]models.ClosedPnl,
	ledger *helpers.Ledger,
	isolated bool,
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

		env := buildEnvelope(ep, ordersByID, ordersBySymbol, closedByOrder, ledger, isolated)

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
	ordersByID map[string]models.Order,
	ordersBySymbol map[string][]models.Order,
	closedByOrder map[string]models.ClosedPnl,
	ledger *helpers.Ledger,
	isolated bool,
) envelope.PositionEnvelope {
	orders := helpers.OrdersForEpisode(ep, ordersByID)
	tp, sl := helpers.TPSLForEpisode(ep, orders, ordersBySymbol[ep.Symbol])

	var funding, refund float64
	if ledger != nil {
		openMs, closeMs := ep.OpenAt.UnixMilli(), ep.CloseAt.UnixMilli()
		funding = ledger.FundingForRange(ep.Symbol, openMs, closeMs)
		refund = ledger.FeeRefundForRange(ep.Symbol, openMs, closeMs)
	}

	return envelope.PositionEnvelope{
		Symbol:     ep.Symbol,
		Side:       ep.Side(),
		Parts:      ep.Parts,
		Orders:     orders,
		OpenAt:     ep.OpenAt,
		CloseAt:    ep.CloseAt,
		PeakSize:   ep.PeakSize,
		OpenSign:   ep.OpenSign,
		Closed:     ep.Closed,
		Leverage:   helpers.LeverageForEpisode(ep, closedByOrder),
		Isolated:   isolated,
		StopLoss:   sl,
		TakeProfit: tp,
		Funding:    funding,
		FeeRefund:  refund,
	}
}

func BalanceSnapshots(
	client *resty.Client,
	ledger *helpers.Ledger,
	windowStart *time.Time,
) ([]domain.UserBalanceSnapshot, error) {
	wallet, err := executors.FetchWalletBalance(client)
	if err != nil {
		return nil, err
	}

	if ledger == nil {
		startMs := int64(0)
		if windowStart != nil {
			startMs = windowStart.UnixMilli()
		}
		rows, err := executors.FetchLedger(client, startMs, 0, executors.LedgerFilter{})
		if err != nil {
			return nil, err
		}
		ledger = helpers.BuildLedger(rows)
	}

	return builders.BuildBalanceSnapshots(executors.TotalWalletBalance(wallet), ledger, windowStart), nil
}

func ReconstructClosedPositions(
	client *resty.Client,
	cutoff *time.Time,
) ([]domain.Position, error) {
	info, err := executors.FetchAccountInfo(client)
	if err != nil {
		return nil, err
	}
	isolated := info.MarginMode == models.MarginModeIsolated

	openPositions, err := executors.FetchOpenPositions(client)
	if err != nil {
		return nil, err
	}

	walk, err := CollectWeeks(client, openPositions, cutoff, false)
	if err != nil {
		return nil, err
	}

	groups := GroupsClosedAfter(walk.Groups, cutoff)
	if len(groups) == 0 {
		return []domain.Position{}, nil
	}

	ledger := helpers.BuildLedger(walk.Entries)

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, candleRequests, defaultCandleWorkers)

	envelopes := make(chan envelope.PositionEnvelope)
	positionsCh := make(chan domain.Position)

	go func() {
		ReconstructPositions(
			groups,
			helpers.IndexOrdersByID(walk.Orders),
			helpers.GroupOrdersBySymbol(walk.Orders),
			helpers.IndexClosedPnlByOrder(walk.Closed),
			ledger,
			isolated,
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

	walk, err := CollectWeeks(client, raw, nil, true)
	if err != nil {
		return nil, err
	}

	return builders.BuildOpenPositions(raw, walk.Fills, helpers.IndexOrdersByID(walk.Orders)), nil
}
