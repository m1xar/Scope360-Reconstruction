package reconstructor

import (
	"sort"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/workers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

// Dataset is every raw MEXC response the builders need, fetched once by
// Load for the requested scope.
type Dataset struct {
	client *resty.Client
	cutoff *time.Time
	scope  scope.Scope

	closed        []models.HistoryPosition // closed inside the window
	contractSizes map[string]float64
	open          []models.OpenPosition

	orders   []models.Order
	ordersOK bool
	funding  []models.FundingRecord

	equity      float64
	equityOK    bool
	transfers   []models.TransferRecord
	transfersOK bool

	closedOnce sync.Once
	closedPos  []domain.Position
	closedErr  error
}

// Load fetches the datasets the scope needs, once each. Balance snapshots
// are built from the closed positions' PnL, so Balances implies Closed.
func Load(client *resty.Client, cutoff *time.Time, s scope.Scope) (*Dataset, error) {
	if s.Has(scope.Balances) {
		s |= scope.Closed
	}
	d := &Dataset{client: client, cutoff: cutoff, scope: s}
	cutoffMs := window.StartMs(cutoff)

	var equityErr error
	err := parallel.Run(
		when(s.Has(scope.Closed), func() error {
			closed, err := executors.FetchAllHistoryPositions(client, cutoffMs)
			if err != nil {
				return err
			}
			d.closed = positionsClosedAfter(closed, cutoff)
			return nil
		}),
		when(s.Has(scope.Open), func() error {
			open, err := executors.FetchOpenPositions(client)
			if err != nil {
				return err
			}
			d.open = open
			return nil
		}),
		when(s.Any(scope.Closed|scope.Balances), func() error {
			equity, err := FetchStableEquity(client)
			if err != nil {
				equityErr = err // closed positions only lose BalanceInit
				return nil
			}
			d.equity, d.equityOK = equity, true
			return nil
		}),
		when(s.Has(scope.Fundings) && !s.Has(scope.Closed), func() error {
			return d.loadFunding(cutoffMs)
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

	// Everything keyed by the positions: orders and funding from the oldest
	// closed position (or the oldest open one), transfers from the earliest
	// of the cutoff and the oldest closed position for the balance curve.
	ordersFrom, fundingFrom, transfersFrom := int64(0), int64(0), cutoffMs
	if len(d.closed) > 0 {
		oldest := d.closed[0].CreateTime
		for _, cp := range d.closed[1:] {
			if cp.CreateTime < oldest {
				oldest = cp.CreateTime
			}
		}
		ordersFrom = oldest - 10*60*1000
		fundingFrom = ordersFrom
		if cutoff != nil && oldest < transfersFrom {
			transfersFrom = oldest
		}
	}
	if s.Has(scope.Fundings) && (cutoff == nil || cutoffMs < fundingFrom) {
		fundingFrom = cutoffMs
	}
	if start := oldestOpenMs(d.open); start > 0 && (ordersFrom == 0 || start < ordersFrom) {
		ordersFrom = start
	}

	var ordersErr error
	err = parallel.Run(
		when(len(d.closed) > 0, func() error {
			d.contractSizes = fetchContractSizes(client, d.closed)
			return nil
		}),
		when(ordersFrom > 0, func() error {
			orders, err := executors.FetchAllHistoryOrders(client, ordersFrom)
			if err != nil {
				ordersErr = err // open positions only lose their orders
				return nil
			}
			d.orders, d.ordersOK = orders, true
			return nil
		}),
		when(s.Has(scope.Closed) && (len(d.closed) > 0 || s.Has(scope.Fundings)), func() error {
			return d.loadFunding(fundingFrom)
		}),
		when(s.Has(scope.Closed) && (len(d.closed) > 0 || s.Any(scope.Transactions|scope.Balances)), func() error {
			if err := d.loadTransfers(transfersFrom); err != nil && s.Any(scope.Transactions|scope.Balances) {
				return err
			}
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}
	if !d.ordersOK && ordersErr != nil && len(d.closed) > 0 {
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

func (d *Dataset) loadFunding(sinceMs int64) error {
	records, err := executors.FetchAllFundingRecords(d.client, sinceMs)
	if err != nil {
		return err
	}
	d.funding = records
	return nil
}

func (d *Dataset) loadTransfers(sinceMs int64) error {
	transfers, err := executors.FetchAllTransferRecords(d.client, sinceMs)
	if err != nil {
		return err
	}
	d.transfers, d.transfersOK = transfers, true
	return nil
}

func oldestOpenMs(open []models.OpenPosition) int64 {
	start := int64(0)
	for _, r := range open {
		if r.HoldVol <= 0 {
			continue
		}
		if start == 0 || r.CreateTime < start {
			start = r.CreateTime
		}
	}
	return start
}

// ClosedPositions builds the positions closed inside the window, with
// MAE/MFE from candles and BalanceInit from transfers and realised PnL.
func (d *Dataset) ClosedPositions() ([]domain.Position, error) {
	d.closedOnce.Do(func() { d.closedPos, d.closedErr = d.buildClosed() })
	return d.closedPos, d.closedErr
}

func (d *Dataset) buildClosed() ([]domain.Position, error) {
	if len(d.closed) == 0 {
		return []domain.Position{}, nil
	}
	ordersBySymbol := helpers.GroupOrdersBySymbol(d.orders)

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(d.client, candleRequests, defaultCandleWorkers)

	type pendingCandle struct {
		idx     int
		replyCh chan helpers.CandleResponse
	}

	pending := make([]pendingCandle, 0, len(d.closed))
	positions := make([]domain.Position, len(d.closed))

	for i, cp := range d.closed {
		posOrders := helpers.MatchOrdersToPosition(cp, ordersBySymbol)
		funding := builders.ExtractFundingForPosition(d.funding, cp.Symbol, cp.CreateTime, cp.UpdateTime)

		pos, err := builders.BuildPosition(cp, posOrders, funding, d.contractSizes[cp.Symbol])
		if err != nil {
			continue
		}
		positions[i] = pos

		replyCh := make(chan helpers.CandleResponse, 1)
		candleRequests <- helpers.CandleRequest{
			Symbol:  cp.Symbol,
			Bar:     "1m",
			StartMs: cp.CreateTime,
			EndMs:   cp.UpdateTime,
			ReplyCh: replyCh,
		}
		pending = append(pending, pendingCandle{idx: i, replyCh: replyCh})
	}
	close(candleRequests)

	for _, p := range pending {
		resp := <-p.replyCh
		if resp.Err == nil {
			high, low := helpers.GetHighLow(resp.Candles)
			helpers.ApplyMAEMFE(&positions[p.idx], high, low)
		}
	}

	filtered := make([]domain.Position, 0, len(positions))
	for _, pos := range positions {
		if pos.ID != uuid.Nil {
			filtered = append(filtered, pos)
		}
	}
	positions = filtered

	sort.Slice(positions, func(i, j int) bool {
		if positions[i].ClosedAt == nil {
			return true
		}
		if positions[j].ClosedAt == nil {
			return false
		}
		return positions[i].ClosedAt.Before(*positions[j].ClosedAt)
	})

	if d.equityOK && d.transfersOK {
		snapshots := builders.BuildBalanceSnapshots(d.equity, d.transfers, positions, helpers.BalanceWindowStart(positions, d.cutoff))
		helpers.AttachBalanceInit(&positions, snapshots)
	}
	return positions, nil
}

// OpenPositions builds the open positions with their orders.
func (d *Dataset) OpenPositions() ([]domain.OpenPosition, error) {
	positions := make([]domain.OpenPosition, 0, len(d.open))
	for _, r := range d.open {
		if r.HoldVol <= 0 {
			continue
		}
		positions = append(positions, builders.BuildOpenPosition(r))
	}
	if d.ordersOK {
		enrichOpenPositionOrders(d.orders, d.open, positions)
	}
	return positions, nil
}

// BalanceSnapshots returns the stable-asset equity after every transfer and
// closed position inside the window, oldest first.
func (d *Dataset) BalanceSnapshots() ([]domain.UserBalanceSnapshot, error) {
	positions, err := d.ClosedPositions()
	if err != nil {
		return nil, err
	}
	snapshots := builders.BuildBalanceSnapshots(d.equity, d.transfers, positions, helpers.BalanceWindowStart(positions, d.cutoff))
	if d.cutoff != nil {
		filtered := snapshots[:0]
		for _, s := range snapshots {
			if !s.CreatedAt.Before(*d.cutoff) {
				filtered = append(filtered, s)
			}
		}
		snapshots = filtered
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})
	return snapshots, nil
}

// CurrentBalance is the stable-asset equity.
func (d *Dataset) CurrentBalance() float64 {
	return d.equity
}

// Transactions are the deposits and withdrawals inside the window.
func (d *Dataset) Transactions() []domain.Transaction {
	transactions := builders.BuildTransactions(d.transfers)
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

// Fundings are the funding payments inside the window.
func (d *Dataset) Fundings() []domain.UserFunding {
	fundings := builders.BuildUserFundings(d.funding)
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
