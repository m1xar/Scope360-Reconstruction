package reconstructor

import (
	"sort"
	"sync"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	connector "github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/models"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/reconstructor/workers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

type Dataset struct {
	client *connector.Client
	cutoff *time.Time
	scope  scope.Scope

	snapshot *models.OrderlyPositionsResponse
	open     []models.OrderlyPosition

	walk   TradeWalk
	groups [][]models.OrderlyTrade
	trades []models.OrderlyTrade

	orders     []models.OrderlyOrder
	algoOrders []models.OrderlyAlgoOrder
	fundings   []models.OrderlyFunding

	assetHistory []models.OrderlyAssetHistory
	markPrices   map[string]float64

	closedOnce sync.Once
	closed     []domain.Position
	closedErr  error
}

func Load(client *connector.Client, cutoff *time.Time, s scope.Scope) (*Dataset, error) {
	if s.Has(scope.Balances) {
		s |= scope.Closed
	}
	d := &Dataset{client: client, cutoff: cutoff, scope: s}
	cutoffMs := window.StartMs(cutoff)

	err := parallel.Run(
		when(s.Any(scope.Closed|scope.Open|scope.Balances), func() error {
			snapshot, err := executors.FetchPositionsSnapshot(client)
			if err != nil {
				return err
			}
			d.snapshot = snapshot
			for _, p := range snapshot.Rows {
				if p.PositionQty != 0 {
					d.open = append(d.open, p)
				}
			}
			return nil
		}),
		when(s.Any(scope.Balances|scope.Transactions), func() error {
			prices, err := executors.FetchMarkPrices(client)
			if err != nil {
				return err
			}
			d.markPrices = prices
			return nil
		}),
		when(s.Has(scope.Transactions) && !s.Has(scope.Balances), func() error {
			return d.loadAssetHistory(cutoffMs)
		}),
		when(s.Has(scope.Fundings) && !s.Has(scope.Closed), func() error {
			return d.loadFundings(cutoffMs)
		}),
	)
	if err != nil {
		return nil, err
	}

	reachMs := int64(0)
	if s.Has(scope.Open) {
		reachMs = oldestOpenMs(d.open)
	}
	if s.Has(scope.Closed) {
		walk, err := collectClosedEpisodes(client, "", d.open, cutoff, reachMs)
		if err != nil {
			return nil, err
		}
		d.walk = walk
		d.trades = walk.Trades
		d.groups = GroupsClosedAfter(walk.Groups, cutoff)
	} else if s.Has(scope.Open) && reachMs > 0 {
		trades, err := executors.FetchAllTrades(client, "", reachMs, 0)
		if err != nil {
			return nil, err
		}
		d.trades = trades
	}

	since := int64(0)
	if len(d.groups) > 0 {
		since = d.walk.EarliestOpenMs()
	}
	ordersFrom := since
	if reachMs > 0 && (ordersFrom == 0 || reachMs < ordersFrom) {
		ordersFrom = reachMs
	}
	fundingsFrom := since
	if s.Has(scope.Fundings) && (cutoff == nil || cutoffMs < fundingsFrom) {
		fundingsFrom = cutoffMs
	}

	return d, parallel.Run(
		when(len(d.groups) > 0 || (s.Has(scope.Open) && len(d.open) > 0), func() error {
			orders, err := executors.FetchFilledOrders(client, "", ordersFrom, 0)
			if err != nil {
				if len(d.groups) > 0 {
					return err
				}
				return nil
			}
			d.orders = orders
			return nil
		}),
		when(len(d.groups) > 0, func() error {
			algoOrders, err := executors.FetchAlgoOrders(client, "", since, 0)
			if err != nil {
				return err
			}
			d.algoOrders = algoOrders
			return nil
		}),
		when(s.Has(scope.Closed) && (len(d.groups) > 0 || s.Has(scope.Fundings)), func() error {
			return d.loadFundings(fundingsFrom)
		}),
		when(s.Has(scope.Balances), func() error {
			from := cutoffMs
			if cutoff != nil && len(d.groups) > 0 && since < from {
				from = since
			}
			return d.loadAssetHistory(from)
		}),
	)
}

func when(cond bool, fn func() error) func() error {
	if !cond {
		return nil
	}
	return fn
}

func (d *Dataset) loadAssetHistory(startMs int64) error {
	history, err := executors.FetchAssetHistory(d.client, startMs, 0)
	if err != nil {
		return err
	}
	d.assetHistory = history
	return nil
}

func (d *Dataset) loadFundings(startMs int64) error {
	fundings, err := executors.FetchAllFunding(d.client, "", startMs, 0)
	if err != nil {
		return err
	}
	d.fundings = fundings
	return nil
}

func oldestOpenMs(open []models.OrderlyPosition) int64 {
	start := int64(0)
	for _, p := range open {
		if p.Timestamp > 0 && (start == 0 || p.Timestamp < start) {
			start = p.Timestamp
		}
	}
	return start
}

func (d *Dataset) ClosedPositions() ([]domain.Position, error) {
	d.closedOnce.Do(func() { d.closed, d.closedErr = d.buildClosed() })
	return d.closed, d.closedErr
}

func (d *Dataset) buildClosed() ([]domain.Position, error) {
	if len(d.groups) == 0 {
		return []domain.Position{}, nil
	}

	orderMap := helpers.BuildOrderMap(d.orders)
	algoIdx := helpers.BuildAlgoOrderIndex(d.algoOrders)

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(d.client, candleRequests, defaultCandleWorkers)

	envelopes := make(chan envelope.TradeEnvelope)
	positionsCh := make(chan domain.Position)

	go func() {
		ReconstructTrades(d.groups, d.fundings, orderMap, algoIdx, candleRequests, envelopes)
		close(envelopes)
		close(candleRequests)
	}()

	workers.StartPositionBuilders(envelopes, positionsCh, defaultPositionWorkers)

	positions := make([]domain.Position, 0)
	for pos := range positionsCh {
		positions = append(positions, pos)
	}

	sort.Slice(positions, func(i, j int) bool {
		iClosedAt := positions[i].ClosedAt
		jClosedAt := positions[j].ClosedAt
		if iClosedAt == nil && jClosedAt == nil {
			return i < j
		}
		if iClosedAt == nil {
			return false
		}
		if jClosedAt == nil {
			return true
		}
		return iClosedAt.Before(*jClosedAt)
	})

	enrichPositionsWithRisk(d.snapshot, positions)

	if d.scope.Has(scope.Balances) {
		snapshots, err := d.rawSnapshots(positions)
		if err != nil {
			return nil, err
		}
		helpers.AttachBalanceInit(&positions, snapshots)
	}
	return helpers.FilterPositionsByClosedAt(positions, d.cutoff), nil
}

func (d *Dataset) rawSnapshots(positions []domain.Position) ([]domain.UserBalanceSnapshot, error) {
	return builders.BuildBalanceSnapshots(
		d.snapshot.AccountValue,
		d.assetHistory,
		positions,
		d.markPrices,
		helpers.BalanceWindowStart(positions, d.cutoff),
	)
}

func (d *Dataset) OpenPositions() ([]domain.OpenPosition, error) {
	positions := builders.BuildOpenPositions(d.open)
	enrichOpenPositionOrders(d.trades, helpers.BuildOrderMap(d.orders), positions)
	return positions, nil
}

func (d *Dataset) BalanceSnapshots() ([]domain.UserBalanceSnapshot, error) {
	positions, err := d.ClosedPositions()
	if err != nil {
		return nil, err
	}
	snapshots, err := d.rawSnapshots(positions)
	if err != nil {
		return nil, err
	}
	snapshots = helpers.FilterBalanceSnapshotsByCreatedAt(snapshots, d.cutoff)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})
	return snapshots, nil
}

func (d *Dataset) CurrentBalance() float64 {
	return helpers.Round8(d.snapshot.AccountValue)
}

func (d *Dataset) Transactions() ([]domain.Transaction, error) {
	transactions, err := builders.BuildTransactions(d.assetHistory, d.markPrices)
	if err != nil {
		return nil, err
	}
	if d.cutoff == nil {
		return transactions, nil
	}
	filtered := transactions[:0]
	for _, tx := range transactions {
		if !tx.Time.Before(*d.cutoff) {
			filtered = append(filtered, tx)
		}
	}
	return filtered, nil
}

func (d *Dataset) Fundings() []domain.UserFunding {
	fundings := make([]domain.UserFunding, 0, len(d.fundings))
	for _, fund := range d.fundings {
		fundings = append(fundings, builders.BuildUserFunding(fund))
	}
	return helpers.FilterFundingsByCreatedAt(fundings, d.cutoff)
}
