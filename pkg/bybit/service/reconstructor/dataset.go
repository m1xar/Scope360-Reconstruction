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

// Dataset is every raw Bybit response the builders need, fetched once by
// Load for the requested scope.
type Dataset struct {
	client *resty.Client
	cutoff *time.Time
	scope  scope.Scope

	isolated bool
	open     []models.Position
	wallet   models.WalletAccount
	walletOK bool

	// walk is the weekly pass over the transaction log, orders and closed
	// PnL; its unfiltered ledger entries also feed balances, transactions
	// and fundings. Scopes without positions load the ledger directly.
	walk   Walk
	ledger *helpers.Ledger
	groups [][]helpers.Fill // closed episodes ending after the cutoff
}

// Load fetches the datasets the scope needs, once each. One weekly walk
// serves closed and open positions: with both in scope it runs past the
// cutoff until every open position is walked back to its opening fill.
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
				walletErr = err // closed positions only lose BalanceInit
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
	// Without a cutoff the closed walk covers the whole retention, opening
	// fills included; otherwise it has to resolve the open positions past
	// the cutoff.
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

// loadLedger fetches the transaction log from startMs for scopes without
// positions: every type for balances, only the types a transactions- or
// fundings-only scope reads.
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

// ClosedPositions builds the positions closed inside the window, with
// MAE/MFE from candles and BalanceInit from the transaction log.
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

// OpenPositions builds the open positions with their opening orders.
func (d *Dataset) OpenPositions() ([]domain.OpenPosition, error) {
	if len(d.open) == 0 {
		return []domain.OpenPosition{}, nil
	}
	return builders.BuildOpenPositions(d.open, d.walk.Fills, helpers.IndexOrdersByID(d.walk.Orders)), nil
}

// BalanceSnapshots returns the wallet balance after every stable-asset
// ledger entry in the window plus the current balance.
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

// CurrentBalance is the unified account's total equity.
func (d *Dataset) CurrentBalance() float64 {
	return helpers.Round8(executors.TotalEquity(d.wallet))
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

// Fundings are the linear settlement payments inside the window.
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
