package reconstructor

import (
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit/models"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor/workers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

type Dataset struct {
	client *resty.Client
	cutoff *time.Time
	scope  scope.Scope

	isolated bool
	open     []models.Position
	wallet   models.WalletAccount
	walletOK bool

	walk   Walk
	ledger *helpers.Ledger
	groups [][]helpers.Fill
}

func Load(client *resty.Client, cutoff *time.Time, s scope.Scope) (*Dataset, error) {
	d := &Dataset{client: client, cutoff: cutoff, scope: s}

	var walletErr error
	err := parallel.Run(
		when(s.Has(scope.Closed), func() error {
			info, err := executors.FetchAccountInfo(client)
			if err != nil {
				return err
			}
			d.isolated = info.MarginMode == models.MarginModeIsolated
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
		when(s.Any(scope.Closed|scope.Balances), func() error {
			wallet, err := executors.FetchWalletBalance(client)
			if err != nil {
				walletErr = err
				return nil
			}
			d.wallet, d.walletOK = wallet, true
			return nil
		}),
		when(!s.Has(scope.Closed) && s.Any(scope.Balances|scope.Transactions|scope.Fundings), func() error {
			ledger, err := d.loadLedger(window.StartMs(cutoff))
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
	if !d.walletOK && s.Has(scope.Balances) {
		return nil, walletErr
	}

	if !s.Has(scope.Closed) && (!s.Has(scope.Open) || len(d.open) == 0) {
		return d, nil
	}
	var walkCutoff *time.Time
	if s.Has(scope.Closed) {
		walkCutoff = cutoff
	}
	untilResolved := s.Has(scope.Open) && !(s.Has(scope.Closed) && cutoff == nil)
	walk, err := CollectWeeks(client, d.open, walkCutoff, untilResolved)
	if err != nil {
		return nil, err
	}
	d.walk = walk
	if s.Has(scope.Closed) {
		d.groups = GroupsClosedAfter(walk.Groups, cutoff)
		d.ledger = helpers.BuildLedger(walk.Entries)
	}
	return d, nil
}

func when(cond bool, fn func() error) func() error {
	if !cond {
		return nil
	}
	return fn
}

func (d *Dataset) loadLedger(startMs int64) (*helpers.Ledger, error) {
	var filters []executors.LedgerFilter
	switch {
	case d.scope.Has(scope.Balances) || d.scope.Has(scope.Transactions|scope.Fundings):
		filters = []executors.LedgerFilter{{}}
	case d.scope.Has(scope.Transactions):
		filters = []executors.LedgerFilter{{Type: models.LedgerTransferIn}, {Type: models.LedgerTransferOut}}
	default:
		filters = []executors.LedgerFilter{{Category: models.CategoryLinear, Type: models.LedgerSettlement}}
	}
	var rows []models.LedgerEntry
	for _, filter := range filters {
		page, err := executors.FetchLedger(d.client, startMs, 0, filter)
		if err != nil {
			return nil, err
		}
		rows = append(rows, page...)
	}
	return helpers.BuildLedger(rows), nil
}

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
			helpers.IndexOrdersByID(d.walk.Orders),
			helpers.GroupOrdersBySymbol(d.walk.Orders),
			helpers.IndexClosedPnlByOrder(d.walk.Closed),
			d.ledger,
			d.isolated,
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

	if d.walletOK {
		snapshots := builders.BuildBalanceSnapshots(executors.TotalWalletBalance(d.wallet), d.ledger, helpers.BalanceWindowStart(positions, d.cutoff))
		helpers.AttachBalanceInit(&positions, snapshots)
	}
	return positions, nil
}

func (d *Dataset) OpenPositions() ([]domain.OpenPosition, error) {
	if len(d.open) == 0 {
		return []domain.OpenPosition{}, nil
	}
	return builders.BuildOpenPositions(d.open, d.walk.Fills, helpers.IndexOrdersByID(d.walk.Orders)), nil
}

func (d *Dataset) BalanceSnapshots() []domain.UserBalanceSnapshot {
	snapshots := builders.BuildBalanceSnapshots(executors.TotalWalletBalance(d.wallet), d.ledger, d.cutoff)
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

func (d *Dataset) CurrentBalance() float64 {
	return helpers.Round8(executors.TotalEquity(d.wallet))
}

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
