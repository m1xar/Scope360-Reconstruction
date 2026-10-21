package reconstructor

import (
	"sort"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/workers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

type Dataset struct {
	client   *resty.Client
	baseURL  string
	cutoff   *time.Time
	scope    scope.Scope
	windowMs int64

	open        []models.OpenPosition
	instruments map[string]models.Instrument

	fills      []models.Fill
	groups     [][]models.Fill
	openFills  map[string][]models.Fill
	earliestMs int64
	orders     []models.Order

	bills   []models.Bill
	billsOK bool
	balance models.Balance
	balOK   bool

	snapOnce  sync.Once
	snapshots []domain.UserBalanceSnapshot

	closedOnce sync.Once
	closed     []domain.Position
	closedErr  error
}

func Load(client *resty.Client, baseURL string, cutoff *time.Time, s scope.Scope) (*Dataset, error) {
	d := &Dataset{client: client, baseURL: baseURL, cutoff: cutoff, scope: s}
	d.windowMs = window.StartMs(cutoff)
	if d.windowMs <= 0 {
		d.windowMs = executors.BillsDefaultStartMs()
	}

	var balErr error
	err := parallel.Run(
		when(s.Any(scope.Closed|scope.Open), func() error {
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

	reachMs := int64(0)
	if s.Has(scope.Open) {
		reachMs = helpers.OldestOpenMs(d.open)
	}
	ordersFrom := reachMs
	if s.Has(scope.Closed) {
		walk, err := collectClosedEpisodes(client, baseURL, d.open, cutoff, reachMs)
		if err != nil {
			return nil, err
		}
		d.fills = walk.Fills
		d.groups = GroupsClosedAfter(walk.Groups, cutoff)
		d.openFills = walk.OpenFills
		d.earliestMs = earliestOpenMs(d.groups)
		if d.earliestMs > 0 && (ordersFrom == 0 || d.earliestMs < ordersFrom) {
			ordersFrom = d.earliestMs
		}
	} else if s.Has(scope.Open) && reachMs > 0 {
		fills, err := executors.FetchAllSwapAndFuturesFills(client, baseURL, reachMs)
		if err != nil {
			return nil, err
		}
		segmenter := helpers.NewFillSegmenter(d.open)
		segmenter.PushOlderBatch(fills)
		d.fills = fills
		d.openFills = segmenter.OpenFills()
	}
	if ordersFrom > 0 {
		ordersFrom -= openPositionOrdersLookback.Milliseconds()
	}

	billsFrom, billsWanted := d.billsRange()
	identifiers := d.identifiers()

	return d, parallel.Run(
		when(ordersFrom > 0 && (len(d.groups) > 0 || len(d.open) > 0), func() error {
			orders, err := executors.FetchAllSwapAndFuturesOrders(client, baseURL, ordersFrom)
			if err != nil {
				return err
			}
			d.orders = orders
			return nil
		}),
		when(billsWanted, func() error {
			bills, err := d.fetchBills(billsFrom)
			if err != nil {
				if s.Any(scope.Balances | scope.Transactions | scope.Fundings) {
					return err
				}
				return nil
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
}

func when(cond bool, fn func() error) func() error {
	if !cond {
		return nil
	}
	return fn
}

func (d *Dataset) billsRange() (int64, bool) {
	from, wanted := int64(0), false
	if d.scope.Any(scope.Balances | scope.Transactions | scope.Fundings) {
		from, wanted = d.windowMs, true
	}
	if d.scope.Has(scope.Closed) && d.earliestMs > 0 {
		oldest := d.earliestMs - billsLookback.Milliseconds()
		if !wanted || oldest < from {
			from = oldest
		}
		wanted = true
	}
	return from, wanted
}

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
	add := func(instID, instType string) {
		if _, ok := identifiers[instID]; !ok {
			identifiers[instID] = models.Instrumentidentifier{InstID: instID, InstType: instType}
		}
	}
	for _, group := range d.groups {
		for _, f := range group {
			add(f.InstId, f.InstType)
		}
	}
	for _, fills := range d.openFills {
		for _, f := range fills {
			add(f.InstId, f.InstType)
		}
	}
	for _, op := range d.open {
		add(op.InstId, op.InstType)
	}
	return identifiers
}

func (d *Dataset) swapFuturesBills() []models.Bill {
	out := make([]models.Bill, 0, len(d.bills))
	for _, b := range d.bills {
		if b.InstType == "SWAP" || b.InstType == "FUTURES" {
			out = append(out, b)
		}
	}
	return out
}

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
			d.groups, helpers.OrdersByID(d.orders), d.swapFuturesBills(), d.instruments,
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

	helpers.AttachBalanceInit(&positions, d.allSnapshots())
	return positions, nil
}

func (d *Dataset) OpenPositions() ([]domain.OpenPosition, error) {
	if len(d.open) == 0 {
		return []domain.OpenPosition{}, nil
	}
	return builders.BuildOpenPositions(d.open, d.openFills, d.fills, helpers.OrdersByID(d.orders), d.instruments), nil
}

func (d *Dataset) BalanceSnapshots() []domain.UserBalanceSnapshot {
	snapshots := d.allSnapshots()
	out := make([]domain.UserBalanceSnapshot, 0, len(snapshots))
	for _, s := range snapshots {
		if s.CreatedAt.UnixMilli() >= d.windowMs {
			out = append(out, s)
		}
	}
	return out
}

func (d *Dataset) CurrentBalance() float64 {
	return helpers.MustFloat(d.balance.TotalEq)
}

func (d *Dataset) Transactions() []domain.Transaction {
	transactions := builders.BuildTransactionsFromBills(d.bills)
	out := make([]domain.Transaction, 0, len(transactions))
	for _, tx := range transactions {
		if tx.Time.UnixMilli() >= d.windowMs {
			out = append(out, tx)
		}
	}
	return out
}

func (d *Dataset) Fundings() []domain.UserFunding {
	fundings := make([]domain.UserFunding, 0)
	for _, b := range d.swapFuturesBills() {
		if b.Type != "8" || helpers.MustInt64(b.Ts) < d.windowMs {
			continue
		}
		amount := helpers.MustFloat(b.BalChg)
		if amount == 0 {
			continue
		}
		fundings = append(fundings, domain.UserFunding{
			Pair:      helpers.NormalizePair(b.InstId),
			Symbol:    b.InstId,
			Amount:    helpers.Round8(amount),
			CreatedAt: helpers.TimeFromMs(b.Ts),
		})
	}
	return fundings
}
