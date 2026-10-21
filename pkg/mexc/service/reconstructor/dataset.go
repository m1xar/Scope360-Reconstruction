package reconstructor

import (
	"sort"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/workers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

type Dataset struct {
	client *resty.Client
	cutoff *time.Time
	scope  scope.Scope

	contractSizes map[string]float64
	open          []models.OpenPosition

	groups    [][]models.Order
	openFills map[string][]models.Order
	funding   []models.FundingRecord

	equity      float64
	equityOK    bool
	transfers   []models.TransferRecord
	transfersOK bool

	closedOnce sync.Once
	closedPos  []domain.Position
	closedErr  error
}

func Load(client *resty.Client, cutoff *time.Time, s scope.Scope) (*Dataset, error) {
	if s.Has(scope.Balances) {
		s |= scope.Closed
	}
	d := &Dataset{client: client, cutoff: cutoff, scope: s, contractSizes: map[string]float64{}}
	cutoffMs := window.StartMs(cutoff)

	var equityErr error
	err := parallel.Run(
		when(s.Any(scope.Closed|scope.Open), func() error {
			open, err := executors.FetchOpenPositions(client)
			if err != nil {
				return err
			}
			d.open = open
			return nil
		}),
		when(s.Any(scope.Closed|scope.Open), func() error {
			d.contractSizes = fetchContractSizes(client)
			return nil
		}),
		when(s.Any(scope.Closed|scope.Balances), func() error {
			equity, err := FetchStableEquity(client)
			if err != nil {
				equityErr = err
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

	if s.Any(scope.Closed | scope.Open) {
		var walkCutoff *time.Time
		if s.Has(scope.Closed) {
			walkCutoff = cutoff
		}
		untilResolved := s.Has(scope.Open) && !(s.Has(scope.Closed) && cutoff == nil)
		walk, err := walkOrders(func(startMs, endMs int64) ([]models.Order, error) {
			return executors.FetchHistoryOrdersWindow(client, startMs, endMs)
		}, d.open, d.contractSizes, walkCutoff, untilResolved)
		if err != nil {
			return nil, err
		}
		fillMissingContractSizes(client, d.contractSizes, walkSymbols(walk, d.open))
		d.openFills = walk.openFills
		if s.Has(scope.Closed) {
			d.groups = GroupsClosedAfter(walk.groups, cutoff)
		}
	}

	fundingFrom, transfersFrom := int64(0), cutoffMs
	if earliest := (orderWalk{groups: d.groups}).earliestOpenMs(); earliest > 0 {
		fundingFrom = earliest - fundingLookback.Milliseconds()
		if cutoff != nil && earliest < transfersFrom {
			transfersFrom = earliest
		}
	}
	if s.Has(scope.Fundings) && (cutoff == nil || fundingFrom == 0 || cutoffMs < fundingFrom) {
		fundingFrom = cutoffMs
	}

	return d, parallel.Run(
		when(s.Has(scope.Closed) && (len(d.groups) > 0 || s.Has(scope.Fundings)), func() error {
			return d.loadFunding(fundingFrom)
		}),
		when(s.Has(scope.Closed) && (len(d.groups) > 0 || s.Any(scope.Transactions|scope.Balances)), func() error {
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

func (d *Dataset) ClosedPositions() ([]domain.Position, error) {
	d.closedOnce.Do(func() { d.closedPos, d.closedErr = d.buildClosed() })
	return d.closedPos, d.closedErr
}

func (d *Dataset) buildClosed() ([]domain.Position, error) {
	if len(d.groups) == 0 {
		return []domain.Position{}, nil
	}

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(d.client, candleRequests, defaultCandleWorkers)

	envelopes := make(chan envelope.PositionEnvelope)
	positionsCh := make(chan domain.Position)

	go func() {
		ReconstructPositions(d.groups, d.funding, d.contractSizes, candleRequests, envelopes)
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
	return builders.BuildOpenPositions(d.open, d.openFills, d.contractSizes), nil
}

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

func (d *Dataset) CurrentBalance() float64 {
	return d.equity
}

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
