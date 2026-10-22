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

	instrumentsMu sync.Mutex
	instruments   helpers.Instruments

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
		// Contract sizes only; a failure leaves ContractSize nil and must not fail the sync.
		when(history, func() error { _, _ = d.loadInstruments(); return nil }),
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

// loadInstruments fetches effective-instruments once per Dataset (the client
// caches the response too). A failed fetch is not remembered, so a later call
// retries.
func (d *Dataset) loadInstruments() (helpers.Instruments, error) {
	d.instrumentsMu.Lock()
	defer d.instrumentsMu.Unlock()
	if d.instruments != nil {
		return d.instruments, nil
	}
	list, err := executors.FetchInstruments(d.ctx, d.client)
	if err != nil {
		return nil, err
	}
	d.instruments = helpers.IndexInstruments(list)
	return d.instruments, nil
}

// knownInstruments returns what Load managed to fetch, without a new request.
func (d *Dataset) knownInstruments() helpers.Instruments {
	d.instrumentsMu.Lock()
	defer d.instrumentsMu.Unlock()
	return d.instruments
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
		helpers.AttachContractSizes(d.positions, d.knownInstruments())

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
	helpers.AttachOpenContractSizes(open, d.knownInstruments())
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

// ContractSizes returns the contract specification of every instrument of the
// account (and of traded symbols missing from the instrument list, with nil
// ContractSize). It fetches effective-instruments if Load did not (scope 0),
// so it works without loading any history — intended for backfills.
func (d *Dataset) ContractSizes() ([]domain.FXSymbolSpec, error) {
	in, err := d.loadInstruments()
	if err != nil {
		return nil, err
	}
	return helpers.SymbolSpecs(in, d.open, d.closed), nil
}
