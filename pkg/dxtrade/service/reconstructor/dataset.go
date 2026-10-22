package reconstructor

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	connector "github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/service/reconstructor/helpers"
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

	account     models.AccountDetails
	isDemo      bool
	metrics     *models.AccountMetrics
	positions   []models.Position
	working     []models.Order
	history     []models.Order
	transfers   []models.CashTransfer
	instruments []models.InstrumentDetails
	lotSizes    map[string]float64
	quotes      map[string]models.Quote

	orders []models.Order
	ledger []helpers.CashEntry

	once      sync.Once
	closed    []domain.FXPosition
	snapshots []domain.UserBalanceSnapshot
}

func Load(ctx context.Context, client *connector.Client, days int, s scope.Scope) (*Dataset, error) {
	account, err := client.Account(ctx)
	if err != nil {
		return nil, err
	}
	d := &Dataset{
		ctx: ctx, client: client, cutoff: fx.CutoffFromDays(days), scope: s, now: time.Now().UTC(),
		account: *account, isDemo: client.IsDemo(account),
	}
	history := s.Any(scope.Closed | scope.Balances | scope.Open)

	err = parallel.Run(
		func() (err error) { d.metrics, err = executors.FetchMetrics(ctx, client); return },
		func() error {
			instruments, err := executors.FetchInstruments(ctx, client)
			if err != nil && !connector.IsStatus(err, http.StatusNotFound) {
				return err
			}
			d.instruments = instruments
			return nil
		},
		func() error {
			list, err := executors.FetchInstrumentList(ctx, client)
			if err != nil && !connector.IsStatus(err, http.StatusNotFound) {
				return err
			}
			d.lotSizes = helpers.LotSizes(list)
			return nil
		},
		when(history, func() (err error) { d.positions, err = executors.FetchPositions(ctx, client); return }),
		when(s.Has(scope.Open), func() (err error) { d.working, err = executors.FetchWorkingOrders(ctx, client); return }),
	)
	if err != nil {
		return nil, err
	}

	from := d.historyStart()
	err = parallel.Run(
		when(history, func() error {
			orders, err := executors.FetchOrderHistory(ctx, client, from)
			if err != nil {
				return err
			}
			openers, err := executors.FetchOrdersByID(ctx, client, helpers.MissingOpeners(orders))
			if err != nil {
				return err
			}
			d.history = helpers.MergeOrders(orders, openers)
			return nil
		}),
		when(s.Any(scope.Closed|scope.Balances|scope.Transactions), func() (err error) {
			d.transfers, err = executors.FetchTransfers(ctx, client, from)
			return
		}),
		when(s.Has(scope.Open), func() error {
			if quotes, err := executors.FetchQuotes(ctx, client, helpers.Symbols(d.positions)); err == nil {
				d.quotes = quotes
			}
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}

	d.orders = helpers.MergeOrders(d.history, d.working)
	d.ledger = helpers.Ledger(d.orders, d.transfers)
	return d, nil
}

func when(ok bool, fn func() error) func() error {
	if !ok {
		return nil
	}
	return fn
}

func (d *Dataset) historyStart() *time.Time {
	if d.cutoff == nil {
		return nil
	}
	from := *d.cutoff
	for _, p := range d.positions {
		if !p.OpenTime.IsZero() && p.OpenTime.Before(from) {
			from = p.OpenTime.Time
		}
	}
	return &from
}

func (d *Dataset) build() {
	d.once.Do(func() {
		positions := builders.BuildFXPositions(d.orders, d.ledger, d.positions, helpers.InstrumentMap(d.instruments), d.lotSizes, d.account.Hedging())
		positions = fx.ClosedAfter(positions, d.cutoff)

		d.snapshots = fx.ReverseSnapshots(d.balance(), helpers.BalanceEvents(d.ledger), d.windowStart(positions), d.now)
		if d.scope.Has(scope.Closed) {
			fx.EnrichMAEMFE(positions, helpers.BarFetcher(d.ctx, d.client), candleWorkers)
		}
		fx.AttachBalanceInit(positions, d.snapshots)
		d.closed = positions
	})
}

func (d *Dataset) windowStart(positions []domain.FXPosition) *time.Time {
	if d.cutoff == nil {
		return nil
	}
	start := *d.cutoff
	for _, p := range positions {
		if p.CreatedAt.Before(start) {
			start = p.CreatedAt
		}
	}
	return &start
}

func (d *Dataset) balance() float64 {
	if d.metrics == nil {
		return 0
	}
	return d.metrics.Balance.Float()
}

func (d *Dataset) ClosedPositions() []domain.FXPosition {
	d.build()
	return d.closed
}

func (d *Dataset) BalanceSnapshots() []domain.UserBalanceSnapshot {
	d.build()
	return d.snapshots
}

func (d *Dataset) OpenPositions() []domain.FXOpenPosition {
	open := builders.BuildOpenPositions(d.positions, d.orders, d.ledger, d.metrics, d.quotes, d.lotSizes, d.account.Hedging())
	fx.EnrichOpenMAEMFE(open, d.now, helpers.BarFetcher(d.ctx, d.client), candleWorkers)
	return open
}

func (d *Dataset) PendingOrders() []domain.FXOrder {
	return builders.BuildPendingOrders(d.working, d.positions, d.lotSizes)
}

func (d *Dataset) AccountInfo() domain.FXAccountInfo {
	info := domain.FXAccountInfo{
		Balance:  d.balance(),
		Leverage: helpers.AccountLeverage(d.instruments),
		Currency: d.account.BaseCurrency,
		IsDemo:   d.isDemo,
	}
	if d.metrics != nil {
		info.Equity = d.metrics.Equity.Float()
		info.Margin = d.metrics.Margin.Float()
		info.FreeMargin = d.metrics.MarginFree.Float()
	}
	return info
}

// ContractSizes returns lotSize (base-asset units per lot) of every
// instrument from /instruments/query, plus traded symbols without one (nil).
// Load always fetches the instrument list, so this makes no request; a 404 on
// that list leaves every ContractSize nil. The error is always nil and exists
// for parity with the other FX connectors.
func (d *Dataset) ContractSizes() ([]domain.FXSymbolSpec, error) {
	return helpers.SymbolSpecs(d.lotSizes, d.positions, d.orders), nil
}

func (d *Dataset) Transactions() []domain.Transaction {
	transactions := builders.BuildTransactions(d.transfers)
	if d.cutoff == nil {
		return transactions
	}
	kept := transactions[:0]
	for _, tx := range transactions {
		if !tx.Time.Before(*d.cutoff) {
			kept = append(kept, tx)
		}
	}
	return kept
}
