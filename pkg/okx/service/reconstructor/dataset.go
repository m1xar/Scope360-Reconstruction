package reconstructor

import (
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

// Dataset is every raw OKX response the builders need, fetched once by Load
// for the requested scope. Each builder is pure apart from the candles that
// closed positions pull for MAE/MFE.
type Dataset struct {
	client  *resty.Client
	baseURL string
	cutoff  *time.Time
	scope   scope.Scope

	closed      []models.ClosedPosition // closed inside the window
	open        []models.OpenPosition
	instruments map[string]models.Instrument

	orders   []models.Order
	fills    []models.Fill
	fillsErr error // fills are optional: whole-order matching is the fallback
	ordersOK bool

	bills   []models.Bill // every instType the scope needs, unfiltered by type
	billsOK bool
	balance models.Balance
	balOK   bool

	snapOnce  sync.Once
	snapshots []domain.UserBalanceSnapshot
}

// Load fetches the datasets the scope needs, once each. Ranges are widened to
// the union of what the requested consumers would fetch on their own: orders,
// fills and bills start at the earliest of the oldest closed position, the
// oldest open position (minus the order lookback) and the cutoff.
func Load(client *resty.Client, baseURL string, cutoff *time.Time, s scope.Scope) (*Dataset, error) {
	d := &Dataset{client: client, baseURL: baseURL, cutoff: cutoff, scope: s}
	cutoffMs := window.StartMs(cutoff)

	// Phase 1: position lists and the balance are independent.
	var balErr error
	err := parallel.Run(
		when(s.Has(scope.Closed), func() error {
			closed, err := executors.FetchAllClosedPositions(client, baseURL, cutoffMs)
			if err != nil {
				return err
			}
			d.closed = closedPositionsAfter(closed, cutoffMs)
			return nil
		}),
		when(s.Has(scope.Open), func() error {
			open, err := executors.FetchOpenPositions(client, baseURL)
			if err != nil {
				return err
			}
			d.open = open
			return nil
		}),
		when(s.Any(scope.Closed|scope.Balances), func() error {
			bal, err := executors.FetchBalance(client, baseURL)
			if err != nil {
				// Closed positions only lose BalanceInit without a balance.
				balErr = err
				return nil
			}
			d.balance, d.balOK = bal, true
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}
	if !d.balOK && s.Has(scope.Balances) {
		return nil, balErr
	}

	// Phase 2: everything keyed by the positions found above.
	ordersFrom, fillsFrom := d.orderRanges()
	billsFrom, billsWanted := d.billsRange(cutoffMs)
	identifiers := d.identifiers()

	var ordersErr error
	err = parallel.Run(
		when(fillsFrom > 0, func() error {
			orders, fills, fillsErr, err := fetchOrdersAndFills(client, baseURL, ordersFrom, fillsFrom)
			if err != nil {
				// Open positions tolerate missing orders; closed ones need them.
				ordersErr = err
				return nil
			}
			d.orders, d.fills, d.fillsErr, d.ordersOK = orders, fills, fillsErr, true
			return nil
		}),
		when(billsWanted, func() error {
			bills, err := d.fetchBills(billsFrom)
			if err != nil {
				if s.Any(scope.Balances | scope.Transactions | scope.Fundings) {
					return err
				}
				return nil // closed positions only lose BalanceInit
			}
			d.bills, d.billsOK = bills, true
			return nil
		}),
		when(len(identifiers) > 0, func() error {
			instruments, err := executors.FetchInstruments(client, baseURL, identifiers)
			if err != nil {
				return err
			}
			d.instruments = instruments
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}
	if !d.ordersOK && ordersErr != nil && s.Has(scope.Closed) && len(d.closed) > 0 {
		return nil, ordersErr
	}
	return d, nil
}

func when(cond bool, fn func() error) func() error {
	if !cond {
		return nil
	}
	return fn
}

// orderRanges returns the start of the order and fill history the scope
// needs, or zeros when no position asks for any.
func (d *Dataset) orderRanges() (ordersFrom, fillsFrom int64) {
	set := func(o, f int64) {
		if ordersFrom == 0 || o < ordersFrom {
			ordersFrom = o
		}
		if fillsFrom == 0 || f < fillsFrom {
			fillsFrom = f
		}
	}
	if d.scope.Has(scope.Closed) && len(d.closed) > 0 {
		// Everything for closed positions is fetched from the open time of
		// the oldest surviving position, not from the cutoff: a position
		// closed inside the window may have been opened long before it.
		oldest := oldestClosedMs(d.closed) - 10*60*1000
		set(oldest, oldest)
	}
	if d.scope.Has(scope.Open) {
		if start := oldestOpenMs(d.open); start > 0 {
			set(start-openPositionOrdersLookback, start)
		}
	}
	return ordersFrom, fillsFrom
}

// billsRange returns where the bill history has to start for the scope: the
// oldest closed position for BalanceInit, the cutoff (or the archive's
// default depth) for snapshots, transactions and fundings.
func (d *Dataset) billsRange(cutoffMs int64) (int64, bool) {
	from, wanted := int64(0), false
	if d.scope.Any(scope.Balances | scope.Transactions | scope.Fundings) {
		from, wanted = cutoffMs, true
		if from <= 0 {
			from = executors.BillsDefaultStartMs()
		}
	}
	if d.scope.Has(scope.Closed) && len(d.closed) > 0 {
		oldest := oldestClosedMs(d.closed) - 10*60*1000
		if !wanted || oldest < from {
			from = oldest
		}
		wanted = true
	}
	return from, wanted
}

// fetchBills loads the widest bill set any consumer in scope needs:
// transactions look at every instType, the others at SWAP and FUTURES only;
// a fundings-only scope keeps the server-side type filter.
func (d *Dataset) fetchBills(from int64) ([]models.Bill, error) {
	billType := ""
	if !d.scope.Any(scope.Closed | scope.Balances | scope.Transactions) {
		billType = "8"
	}
	if d.scope.Has(scope.Transactions) {
		return executors.FetchAllBills(d.client, d.baseURL, "", from, billType)
	}
	return executors.FetchAllSwapAndFuturesBills(d.client, d.baseURL, from, billType)
}

func (d *Dataset) identifiers() map[string]models.Instrumentidentifier {
	identifiers := map[string]models.Instrumentidentifier{}
	for _, cp := range d.closed {
		if _, ok := identifiers[cp.InstId]; !ok {
			identifiers[cp.InstId] = models.Instrumentidentifier{InstID: cp.InstId, InstType: cp.InstType}
		}
	}
	for _, op := range d.open {
		if _, ok := identifiers[op.InstId]; !ok {
			identifiers[op.InstId] = models.Instrumentidentifier{InstID: op.InstId, InstType: op.InstType}
		}
	}
	return identifiers
}

func oldestClosedMs(closed []models.ClosedPosition) int64 {
	oldest := helpers.MustInt64(closed[0].CTime)
	for _, cp := range closed[1:] {
		if t := helpers.MustInt64(cp.CTime); t < oldest {
			oldest = t
		}
	}
	return oldest
}

func oldestOpenMs(open []models.OpenPosition) int64 {
	start := int64(0)
	for _, r := range open {
		t := helpers.MustInt64(r.CTime)
		if t == 0 {
			continue
		}
		if start == 0 || t < start {
			start = t
		}
	}
	return start
}

// swapFuturesBills narrows the loaded bills to the derivatives accounts the
// balance snapshots and fundings are built from.
func (d *Dataset) swapFuturesBills() []models.Bill {
	out := make([]models.Bill, 0, len(d.bills))
	for _, b := range d.bills {
		if b.InstType == "SWAP" || b.InstType == "FUTURES" {
			out = append(out, b)
		}
	}
	return out
}

// allSnapshots builds the balance snapshots over the whole loaded bill
// range once; BalanceInit needs the ones before the cutoff too.
func (d *Dataset) allSnapshots() []domain.UserBalanceSnapshot {
	d.snapOnce.Do(func() {
		if !d.balOK || !d.billsOK {
			return
		}
		bills := d.swapFuturesBills()
		if len(bills) == 0 {
			return
		}
		d.snapshots = builders.BuildBalanceSnapshotsFromBills(helpers.MustFloat(d.balance.TotalEq), bills)
	})
	return d.snapshots
}

// ClosedPositions builds the positions closed inside the window, with
// MAE/MFE from candles and BalanceInit from the bill history.
func (d *Dataset) ClosedPositions() ([]domain.Position, error) {
	if len(d.closed) == 0 {
		return []domain.Position{}, nil
	}
	positions := buildClosedPositions(d.client, d.baseURL, d.closed, d.orders, d.fills, d.fillsErr, d.instruments)
	helpers.AttachBalanceInit(&positions, d.allSnapshots())
	return positions, nil
}

// OpenPositions builds the open positions with their opening orders.
func (d *Dataset) OpenPositions() ([]domain.OpenPosition, error) {
	positions := make([]domain.OpenPosition, 0, len(d.open))
	for _, r := range d.open {
		positions = append(positions, builders.BuildOpenPosition(r, d.instruments[r.InstId]))
	}
	if d.ordersOK {
		enrichOpenPositionOrders(d.orders, d.fills, d.fillsErr, d.open, positions, d.instruments)
	}
	return positions, nil
}

// BalanceSnapshots returns the USD balance after every derivatives bill in
// the window plus the current balance.
func (d *Dataset) BalanceSnapshots() []domain.UserBalanceSnapshot {
	snapshots := d.allSnapshots()
	if d.cutoff == nil {
		return append([]domain.UserBalanceSnapshot{}, snapshots...)
	}
	out := make([]domain.UserBalanceSnapshot, 0, len(snapshots))
	for _, s := range snapshots {
		if !s.CreatedAt.Before(*d.cutoff) {
			out = append(out, s)
		}
	}
	return out
}

// CurrentBalance is the account's total equity.
func (d *Dataset) CurrentBalance() float64 {
	return helpers.MustFloat(d.balance.TotalEq)
}

// Transactions are the transfers between the funding and trading accounts
// inside the window.
func (d *Dataset) Transactions() []domain.Transaction {
	transactions := builders.BuildTransactionsFromBills(d.bills)
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

// Fundings are the non-zero funding-fee bills inside the window.
func (d *Dataset) Fundings() []domain.UserFunding {
	cutoffMs := window.StartMs(d.cutoff)
	fundings := make([]domain.UserFunding, 0)
	for _, b := range d.swapFuturesBills() {
		if b.Type != "8" {
			continue
		}
		if cutoffMs > 0 && helpers.MustInt64(b.Ts) < cutoffMs {
			continue
		}
		amount := helpers.MustFloat(b.BalChg)
		if amount == 0 {
			continue
		}
		fundings = append(fundings, domain.UserFunding{
			Pair:      helpers.NormalizePair(b.InstId),
			Amount:    helpers.Round8(amount),
			CreatedAt: helpers.TimeFromMs(b.Ts),
		})
	}
	return fundings
}
