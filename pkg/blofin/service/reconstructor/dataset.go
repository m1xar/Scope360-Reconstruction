package reconstructor

import (
	"sort"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/workers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

type Dataset struct {
	client  *resty.Client
	baseURL string
	cutoff  *time.Time
	scope   scope.Scope

	instruments map[string]models.Instrument
	open        []models.OpenPosition

	fills  []models.Fill
	groups [][]models.Fill
	orders []models.Order

	fundingFees []models.FundingFee
	equity      float64
	equityOK    bool
	transfers   []models.Transfer
	transfersOK bool

	closedOnce sync.Once
	closed     []domain.Position
	closedErr  error
}

func Load(client *resty.Client, baseURL string, cutoff *time.Time, s scope.Scope) (*Dataset, error) {
	if s.Has(scope.Balances) {
		s |= scope.Closed
	}
	d := &Dataset{client: client, baseURL: baseURL, cutoff: cutoff, scope: s}
	cutoffMs := window.StartMs(cutoff)

	var equityErr error
	err := parallel.Run(
		when(s.Any(scope.Closed|scope.Open), func() error {
			instruments, err := executors.FetchInstruments(client, baseURL)
			if err != nil {
				return err
			}
			d.instruments = instruments
			return nil
		}),
		when(s.Any(scope.Closed|scope.Open), func() error {
			open, err := executors.FetchOpenPositions(client, baseURL)
			if err != nil {
				return err
			}
			d.open = open
			return nil
		}),
		when(s.Any(scope.Closed|scope.Balances), func() error {
			equity, err := executors.FetchTotalEquity(client, baseURL)
			if err != nil {
				equityErr = err
				return nil
			}
			d.equity, d.equityOK = equity, true
			return nil
		}),
		when(s.Any(scope.Closed|scope.Fundings), func() error {
			fees, err := executors.FetchAllFundingFees(client, baseURL, cutoffMs)
			if err != nil {
				if s.Has(scope.Fundings) {
					return err
				}
				return nil
			}
			d.fundingFees = fees
			return nil
		}),
		when(s.Has(scope.Transactions) && !s.Has(scope.Closed), func() error {
			return d.loadTransfers(cutoffMs)
		}),
	)
	if err != nil {
		return nil, err
	}
	if !d.equityOK && s.Has(scope.Balances) {
		return nil, equityErr
	}

	ordersFrom := int64(0)
	if s.Has(scope.Open) {
		if oldest := helpers.OldestPositionMs(d.open); oldest > 0 {
			ordersFrom = oldest - historyLookback.Milliseconds()
		}
	}
	if s.Has(scope.Closed) {
		walk, err := collectClosedEpisodes(client, baseURL, d.instruments, d.open, cutoff, ordersFrom)
		if err != nil {
			return nil, err
		}
		d.fills = walk.Fills
		d.groups = GroupsClosedAfter(walk.Groups, cutoff)
		if len(d.groups) > 0 {
			oldest := walk.EarliestOpenMs() - historyLookback.Milliseconds()
			if ordersFrom == 0 || oldest < ordersFrom {
				ordersFrom = oldest
			}
		}
	} else if s.Has(scope.Open) && ordersFrom > 0 {
		fills, err := executors.FetchAllFills(client, baseURL, ordersFrom)
		if err != nil {
			return nil, err
		}
		d.fills = fills
	}

	transfersFrom, transfersWanted := cutoffMs, s.Any(scope.Transactions|scope.Balances)
	if len(d.groups) > 0 {
		earliest := helpers.MustInt64(d.groups[0][0].Ts)
		for _, g := range d.groups {
			if ts := helpers.MustInt64(g[0].Ts); ts < earliest {
				earliest = ts
			}
		}
		if cutoff != nil && earliest < transfersFrom {
			transfersFrom = earliest
		}
		transfersWanted = true
	}

	return d, parallel.Run(
		when(ordersFrom > 0 && (len(d.groups) > 0 || len(d.open) > 0), func() error {
			orders, err := executors.FetchAllOrders(client, baseURL, ordersFrom)
			if err != nil {
				return err
			}
			d.orders = orders
			return nil
		}),
		when(transfersWanted && s.Has(scope.Closed), func() error {
			if err := d.loadTransfers(transfersFrom); err != nil && s.Any(scope.Transactions|scope.Balances) {
				return err
			}
			return nil
		}),
	)
}

func when(cond bool, fn func() error) func() error {
	if !cond {
		return nil
	}
	return fn
}

func (d *Dataset) loadTransfers(startMs int64) error {
	transfers, err := executors.FetchAllTransfers(d.client, d.baseURL, startMs)
	if err != nil {
		return err
	}
	d.transfers, d.transfersOK = transfers, true
	return nil
}

func (d *Dataset) ClosedPositions() ([]domain.Position, error) {
	d.closedOnce.Do(func() { d.closed, d.closedErr = d.buildClosed() })
	return d.closed, d.closedErr
}

func (d *Dataset) buildClosed() ([]domain.Position, error) {
	if len(d.groups) == 0 {
		return []domain.Position{}, nil
	}

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(d.client, d.baseURL, candleRequests, defaultCandleWorkers)

	envelopes := make(chan envelope.PositionEnvelope)
	positionsCh := make(chan domain.Position)

	go func() {
		ReconstructPositions(
			d.groups, helpers.IndexOrdersByID(d.orders), d.fundingFees, d.instruments,
			candleRequests, envelopes,
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

	if d.equityOK && d.transfersOK {
		snapshots := builders.BuildBalanceSnapshots(d.equity, d.transfers, positions, helpers.BalanceWindowStart(positions, d.cutoff))
		helpers.AttachBalanceInit(&positions, snapshots)
	}
	return positions, nil
}

func (d *Dataset) OpenPositions() ([]domain.OpenPosition, error) {
	if len(d.open) == 0 {
		return []domain.OpenPosition{}, nil
	}
	return builders.BuildOpenPositions(d.open, d.fills, helpers.IndexOrdersByID(d.orders), d.instruments), nil
}

func (d *Dataset) BalanceSnapshots() ([]domain.UserBalanceSnapshot, error) {
	positions, err := d.ClosedPositions()
	if err != nil {
		return nil, err
	}
	snapshots := builders.BuildBalanceSnapshots(d.equity, d.transfers, positions, helpers.BalanceWindowStart(positions, d.cutoff))
	if d.cutoff == nil {
		return snapshots, nil
	}
	filtered := snapshots[:0]
	for _, s := range snapshots {
		if !s.CreatedAt.Before(*d.cutoff) {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

func (d *Dataset) CurrentBalance() float64 {
	return d.equity
}

func (d *Dataset) Transactions() []domain.Transaction {
	transactions := builders.BuildTransactionsFromTransfers(d.transfers)
	if d.cutoff == nil {
		return transactions
	}
	filtered := transactions[:0]
	for _, tx := range transactions {
		if !tx.Time.Before(*d.cutoff) {
			filtered = append(filtered, tx)
		}
	}
	return filtered
}

func (d *Dataset) Fundings() []domain.UserFunding {
	fundings := builders.BuildFundings(d.fundingFees)
	if d.cutoff == nil {
		return fundings
	}
	filtered := fundings[:0]
	for _, f := range fundings {
		if !f.CreatedAt.Before(*d.cutoff) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}
