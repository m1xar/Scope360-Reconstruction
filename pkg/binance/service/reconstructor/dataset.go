package reconstructor

import (
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/connector/binance/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/connector/binance/models"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/service/reconstructor/workers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

// Dataset is every raw Binance response the builders need, fetched once by
// Load for the requested scope.
type Dataset struct {
	client *resty.Client
	cutoff *time.Time
	scope  scope.Scope

	account   models.Account
	accountOK bool
	open      []models.PositionRisk
	symbolCfg map[string]models.SymbolConfig

	// ledger holds the income rows from the cutoff, extended back to the
	// open of the oldest surviving closed position. A transactions-only or
	// fundings-only scope keeps the server-side type filter.
	ledger *helpers.Ledger

	groups    [][]models.Trade // closed episodes ending after the cutoff
	openFills []models.Trade   // opening fills of the open positions
	orders    []models.Order
}

// Load fetches the datasets the scope needs, once each. One walk over the
// trades serves closed and open positions: with both in scope it runs past
// the cutoff until every open position is walked back to its opening fill.
func Load(client *resty.Client, cutoff *time.Time, s scope.Scope) (*Dataset, error) {
	d := &Dataset{client: client, cutoff: cutoff, scope: s}
	ledgerStartMs := window.StartMs(cutoff)

	var accountErr error
	err := parallel.Run(
		when(s.Any(scope.Closed|scope.Balances), func() error {
			account, err := executors.FetchAccount(client)
			if err != nil {
				accountErr = err // closed positions only lose BalanceInit
				return nil
			}
			d.account, d.accountOK = account, true
			return nil
		}),
		when(s.Any(scope.Closed|scope.Open), func() error {
			open, err := executors.FetchOpenPositions(client)
			if err != nil {
				return err
			}
			d.open = open
			return nil
		}),
		when(s.Any(scope.Closed|scope.Open), func() error {
			d.symbolCfg = fetchSymbolConfigLenient(client)
			return nil
		}),
		when(s.Any(scope.Closed|scope.Balances|scope.Transactions|scope.Fundings), func() error {
			ledger, err := d.loadLedger(ledgerStartMs)
			if err != nil {
				return err
			}
			d.ledger = ledger
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}
	if !d.accountOK && s.Has(scope.Balances) {
		return nil, accountErr
	}
	if !s.Any(scope.Closed | scope.Open) {
		return d, nil
	}

	// Any position closed after the cutoff left REALIZED_PNL/COMMISSION
	// income after it, so the ledger's symbols plus the open positions are
	// every symbol worth walking.
	var symbols []string
	if s.Has(scope.Closed) {
		symbols = unionSymbols(d.ledger, d.open)
	} else {
		symbols = unionSymbols(nil, d.open)
	}
	if len(symbols) == 0 {
		return d, nil
	}

	var walkCutoff *time.Time
	if s.Has(scope.Closed) {
		walkCutoff = cutoff
	}
	// Without a cutoff the closed walk already covers the whole history by
	// id, opening fills included; otherwise the walk has to resolve the
	// open positions past the cutoff.
	untilResolved := s.Has(scope.Open) && !(s.Has(scope.Closed) && cutoff == nil)
	walks, err := collectWalks(client, symbols, d.open, walkCutoff, untilResolved)
	if err != nil {
		return nil, err
	}
	for _, w := range walks {
		d.groups = append(d.groups, w.groups...)
		d.openFills = append(d.openFills, w.openFills...)
	}
	if s.Has(scope.Closed) {
		d.groups = GroupsClosedAfter(d.groups, cutoff)
	} else {
		d.groups = nil
	}
	if len(d.groups) == 0 && len(d.openFills) == 0 {
		return d, nil
	}

	// One fee-conversion pass over every fill; the converter caches klines
	// across symbols.
	normalizeGroupFees(client, append(append([][]models.Trade{}, d.groups...), d.openFills))

	return d, parallel.Run(
		when(cutoff != nil && len(d.groups) > 0, func() error {
			// A surviving position may have opened before the cutoff: extend
			// the ledger back to its open for funding, insurance fees and
			// balance.
			earliestMs := d.groups[0][0].Time
			for _, g := range d.groups {
				if g[0].Time < earliestMs {
					earliestMs = g[0].Time
				}
			}
			if earliestMs >= ledgerStartMs {
				return nil
			}
			// Income is fetched with inclusive bounds: stop just before the
			// range the ledger already holds.
			earlier, err := executors.FetchAllIncome(client, earliestMs, ledgerStartMs-1, "")
			if err != nil {
				return err
			}
			d.ledger = helpers.BuildLedger(append(earlier, d.ledger.Entries...))
			return nil
		}),
		func() error {
			orders, err := collectOrders(client, append(groupFills(d.groups), d.openFills...))
			if err != nil {
				return err
			}
			d.orders = orders
			return nil
		},
	)
}

func when(cond bool, fn func() error) func() error {
	if !cond {
		return nil
	}
	return fn
}

// loadLedger fetches the income rows from startMs: every type when the
// scope builds positions or balances, only the types a transactions- or
// fundings-only scope reads.
func (d *Dataset) loadLedger(startMs int64) (*helpers.Ledger, error) {
	switch {
	case d.scope.Any(scope.Closed|scope.Balances) || d.scope.Has(scope.Transactions|scope.Fundings):
		return LoadLedger(d.client, startMs)
	case d.scope.Has(scope.Transactions):
		rows, err := executors.FetchAllIncome(d.client, startMs, 0, models.IncomeTransfer)
		if err != nil {
			return nil, err
		}
		return helpers.BuildLedger(rows), nil
	default:
		rows, err := executors.FetchAllIncome(d.client, startMs, 0, models.IncomeFundingFee)
		if err != nil {
			return nil, err
		}
		if special, err := executors.FetchAllIncome(d.client, startMs, 0, models.IncomeSpecialFunding); err == nil {
			rows = append(rows, special...)
		}
		return helpers.BuildLedger(rows), nil
	}
}

// ClosedPositions builds the positions closed inside the window, with
// MAE/MFE from candles and BalanceInit from the income ledger.
func (d *Dataset) ClosedPositions() ([]domain.Position, error) {
	if len(d.groups) == 0 {
		return []domain.Position{}, nil
	}

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(d.client, candleRequests, defaultCandleWorkers)

	envelopes := make(chan envelope.PositionEnvelope)
	positionsCh := make(chan domain.Position)

	go func() {
		ReconstructPositions(
			d.groups,
			helpers.IndexOrdersByID(d.orders),
			helpers.GroupOrdersBySymbol(d.orders),
			d.ledger,
			d.symbolCfg,
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

	if d.accountOK {
		snapshots := builders.BuildBalanceSnapshots(executors.StableWalletBalance(d.account), d.ledger, helpers.BalanceWindowStart(positions, d.cutoff))
		helpers.AttachBalanceInit(&positions, snapshots)
	}
	return positions, nil
}

// OpenPositions builds the open positions with their opening orders.
func (d *Dataset) OpenPositions() ([]domain.OpenPosition, error) {
	if len(d.open) == 0 {
		return []domain.OpenPosition{}, nil
	}
	return builders.BuildOpenPositions(d.open, d.openFills, helpers.IndexOrdersByID(d.orders), d.symbolCfg), nil
}

// BalanceSnapshots returns the stable-asset wallet balance after every
// income row in the window plus the current balance.
func (d *Dataset) BalanceSnapshots() []domain.UserBalanceSnapshot {
	snapshots := builders.BuildBalanceSnapshots(executors.StableWalletBalance(d.account), d.ledger, d.cutoff)
	if d.cutoff == nil {
		return snapshots
	}
	out := make([]domain.UserBalanceSnapshot, 0, len(snapshots))
	for _, s := range snapshots {
		if !s.CreatedAt.Before(*d.cutoff) {
			out = append(out, s)
		}
	}
	return out
}

// CurrentBalance is the account's total margin balance.
func (d *Dataset) CurrentBalance() float64 {
	return helpers.Round8(executors.TotalMarginBalance(d.account))
}

// Transactions are the stable-asset transfers inside the window.
func (d *Dataset) Transactions() []domain.Transaction {
	transactions := builders.BuildTransactions(d.ledger)
	if d.cutoff == nil {
		return transactions
	}
	out := make([]domain.Transaction, 0, len(transactions))
	for _, tx := range transactions {
		if !tx.Time.Before(*d.cutoff) {
			out = append(out, tx)
		}
	}
	return out
}

// Fundings are the funding payments inside the window.
func (d *Dataset) Fundings() []domain.UserFunding {
	fundings := builders.BuildFundings(d.ledger)
	if d.cutoff == nil {
		return fundings
	}
	out := make([]domain.UserFunding, 0, len(fundings))
	for _, f := range fundings {
		if !f.CreatedAt.Before(*d.cutoff) {
			out = append(out, f)
		}
	}
	return out
}
