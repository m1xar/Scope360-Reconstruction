package reconstructor

import (
	"context"
	"sync"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	connector "github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
)

const candleWorkers = 2

type Dataset struct {
	ctx    context.Context
	client *connector.Client
	cutoff *time.Time
	scope  scope.Scope
	now    time.Time

	session   *connector.Session
	balance   *models.Balance
	open      []models.OpenPosition
	pending   []models.ActiveOrder
	closed    []models.ClosedPosition
	finance   []models.FinanceOperation
	financeOK bool

	once      sync.Once
	positions []domain.FXPosition
	events    []fx.BalanceEvent
	synthetic *fx.BalanceEvent
}

func Load(ctx context.Context, client *connector.Client, days int, s scope.Scope) (*Dataset, error) {
	d := &Dataset{ctx: ctx, client: client, cutoff: fx.CutoffFromDays(days), scope: s, now: time.Now().UTC()}

	session, err := client.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	d.session = session

	history := s.Any(scope.Closed | scope.Open | scope.Balances | scope.Transactions)
	money := s.Any(scope.Closed | scope.Balances | scope.Transactions)

	err = parallel.Run(
		func() (err error) { d.balance, err = executors.FetchBalance(ctx, client); return },
		when(history, func() (err error) { d.open, err = executors.FetchOpenPositions(ctx, client); return }),
		when(s.Has(scope.Open), func() (err error) { d.pending, err = executors.FetchActiveOrders(ctx, client); return }),
	)
	if err != nil {
		return nil, err
	}
	floor := d.historyFloor()
	err = parallel.Run(
		when(history, func() (err error) {
			d.closed, err = executors.FetchClosedPositions(ctx, client, d.now, floor)
			return
		}),
		when(money, func() (err error) {
			d.finance, d.financeOK, err = executors.FetchFinance(ctx, client, d.now, floor)
			return
		}),
	)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func when(ok bool, fn func() error) func() error {
	if !ok {
		return nil
	}
	return fn
}

func (d *Dataset) historyFloor() time.Time {
	if d.cutoff == nil {
		if d.session.Created.IsZero() {
			return time.Time{}
		}
		return d.session.Created.Add(-24 * time.Hour)
	}
	floor := *d.cutoff
	for _, p := range d.open {
		if t := p.OpenedAt(); !t.IsZero() && t.Before(floor) {
			floor = t.Add(-time.Minute)
		}
	}
	return floor
}

func (d *Dataset) build() {
	d.once.Do(func() {
		openIDs := make(map[string]bool, len(d.open))
		for _, p := range d.open {
			openIDs[p.ID] = true
		}
		d.positions = builders.BuildPositions(d.closed, openIDs, helpers.Leverage(d.session))

		d.events = builders.BalanceEvents(d.closed, d.finance)
		if !d.financeOK {
			if dep, ok := builders.SyntheticDeposit(d.balance.Balance.Float(), d.events, d.session.Created); ok {
				d.synthetic = &dep
				d.events = append(d.events, dep)
			}
		}
		fx.AttachBalanceInit(d.positions, fx.ReverseSnapshots(d.balance.Balance.Float(), d.events, nil, d.now))
		if d.scope.Has(scope.Closed) {
			fx.EnrichMAEMFE(d.positions, helpers.BarFetcher(d.ctx, d.client), candleWorkers)
		}
		d.positions = fx.ClosedAfter(d.positions, d.cutoff)
	})
}

func (d *Dataset) ClosedPositions() []domain.FXPosition {
	d.build()
	return d.positions
}

func (d *Dataset) OpenPositions() []domain.FXOpenPosition {
	open := builders.BuildOpenPositions(d.open, d.closed)
	fx.EnrichOpenMAEMFE(open, d.now, helpers.BarFetcher(d.ctx, d.client), candleWorkers)
	return open
}

func (d *Dataset) PendingOrders() []domain.FXOrder {
	return builders.BuildPendingOrders(d.pending)
}

func (d *Dataset) BalanceSnapshots() []domain.UserBalanceSnapshot {
	d.build()
	var start *time.Time
	if d.cutoff != nil {
		s := *d.cutoff
		for _, p := range d.positions {
			if p.CreatedAt.Before(s) {
				s = p.CreatedAt
			}
		}
		start = &s
	}
	return fx.ReverseSnapshots(d.balance.Balance.Float(), d.events, start, d.now)
}

func (d *Dataset) AccountInfo() domain.FXAccountInfo {
	return helpers.AccountInfo(d.session, d.balance)
}

func (d *Dataset) Transactions() []domain.Transaction {
	d.build()
	txs := builders.BuildTransactions(d.finance, d.synthetic)
	if d.cutoff == nil {
		return txs
	}
	kept := txs[:0]
	for _, tx := range txs {
		if !tx.Time.Before(*d.cutoff) {
			kept = append(kept, tx)
		}
	}
	return kept
}
