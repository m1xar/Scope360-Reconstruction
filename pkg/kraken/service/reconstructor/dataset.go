package reconstructor

import (
	"sort"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
)

type Dataset struct {
	client *resty.Client
	cutoff *time.Time
	scope  scope.Scope

	open    []models.OpenPosition
	tickers map[string]models.Ticker

	walk           FillWalk
	groups         [][]models.Fill
	positionEvents []models.PositionEventElement

	logs   []models.AccountLog
	logsOK bool

	accounts   models.AccountsResponse
	accountsOK bool

	pairBySymbol map[string]string
}

func Load(client *resty.Client, cutoff *time.Time, s scope.Scope) (*Dataset, error) {
	d := &Dataset{client: client, cutoff: cutoff, scope: s}

	var logSince time.Time
	logWanted := s.Any(scope.Balances | scope.Transactions | scope.Fundings)
	if logWanted && cutoff != nil {
		logSince = *cutoff
	}

	err := parallel.Run(
		when(s.Any(scope.Closed|scope.Open), func() error {
			open, err := executors.FetchOpenPositions(client)
			if err != nil {
				return err
			}
			d.open = open
			return nil
		}),
		when(s.Any(scope.Closed|scope.Open|scope.Fundings), func() error {
			tickers, err := executors.FetchTickers(client)
			if err != nil {
				if s.Has(scope.Open) {
					return err
				}
				return nil
			}
			d.tickers = make(map[string]models.Ticker, len(tickers))
			for _, ticker := range tickers {
				d.tickers[strings.ToUpper(ticker.Symbol)] = ticker
			}
			return nil
		}),
		when(s.Has(scope.Balances), func() error {
			accounts, err := executors.FetchAccounts(client)
			if err != nil {
				return nil
			}
			d.accounts, d.accountsOK = accounts, true
			return nil
		}),
		when(logWanted && !s.Has(scope.Closed), func() error {
			return d.loadLogs(logSince)
		}),
	)
	if err != nil {
		return nil, err
	}

	if s.Has(scope.Closed) {
		walk, err := collectClosedEpisodes(client, d.open, cutoff)
		if err != nil {
			return nil, err
		}
		d.walk = walk
		d.groups = GroupsClosedAfter(walk.Groups, cutoff)

		since := logSince
		if len(d.groups) > 0 {
			earliest := walk.EarliestOpen()
			if !logWanted || (cutoff != nil && earliest.Before(since)) {
				since = earliest
			}
		}
		err = parallel.Run(
			when(len(d.groups) > 0, func() error {
				events, err := executors.FetchAllPositionEventsSince(client, walk.EarliestOpen())
				if err != nil {
					return err
				}
				d.positionEvents = events
				return nil
			}),
			when(len(d.groups) > 0 || logWanted, func() error {
				if err := d.loadLogs(since); err != nil && logWanted {
					return err
				}
				return nil
			}),
		)
		if err != nil {
			return nil, err
		}
	}

	d.pairBySymbol = d.buildPairMap()
	return d, nil
}

func when(cond bool, fn func() error) func() error {
	if !cond {
		return nil
	}
	return fn
}

func (d *Dataset) loadLogs(since time.Time) error {
	logs, err := executors.FetchAllAccountLogSince(d.client, since)
	if err != nil {
		return err
	}
	d.logs, d.logsOK = logs, true
	return nil
}

func (d *Dataset) buildPairMap() map[string]string {
	symbols := helpers.SymbolsFromFillsAndEvents(d.walk.Fills, d.positionEvents)
	if d.scope.Has(scope.Fundings) {
		symbols = append(symbols, helpers.SymbolsFromAccountLogs(d.logs)...)
	}
	out := make(map[string]string, len(symbols))
	var missing []string
	for _, symbol := range symbols {
		sym := strings.ToUpper(strings.TrimSpace(symbol))
		if sym == "" {
			continue
		}
		if _, ok := out[sym]; ok {
			continue
		}
		if ticker, ok := d.tickers[sym]; ok && ticker.Pair != "" {
			out[sym] = helpers.NormalizePairText(ticker.Pair)
			continue
		}
		missing = append(missing, sym)
	}
	for sym, pair := range BuildPairMap(d.client, missing) {
		out[sym] = pair
	}
	return out
}

func (d *Dataset) ClosedPositions() ([]domain.Position, error) {
	if len(d.groups) == 0 {
		return []domain.Position{}, nil
	}

	positions, err := builders.BuildClosedPositions(d.groups, d.positionEvents, d.pairBySymbol)
	if err != nil {
		return nil, err
	}

	symbols := helpers.SymbolsFromFillsAndEvents(d.walk.Fills, d.positionEvents)
	EnrichMAEMFE(d.client, &positions, helpers.RawSymbolByPair(symbols, d.pairBySymbol))

	if d.cutoff != nil {
		filtered := positions[:0]
		for _, pos := range positions {
			if pos.ClosedAt != nil && !pos.ClosedAt.Before(*d.cutoff) {
				filtered = append(filtered, pos)
			}
		}
		positions = filtered
	}

	if d.logsOK {
		helpers.AttachBalanceInit(&positions, builders.BuildBalanceSnapshots(d.logs))
	}
	return positions, nil
}

func (d *Dataset) OpenPositions() ([]domain.OpenPosition, error) {
	out := make([]domain.OpenPosition, 0, len(d.open))
	for _, pos := range d.open {
		if pos.Size.Float64() <= 0 {
			continue
		}
		out = append(out, builders.BuildOpenPosition(pos, d.tickers[strings.ToUpper(pos.Symbol)]))
	}
	EnrichOpenPositionOrders(d.client, d.open, out)
	return out, nil
}

func (d *Dataset) BalanceSnapshots() []domain.UserBalanceSnapshot {
	snapshots := builders.BuildBalanceSnapshots(d.logs)
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
	return snapshots
}

func (d *Dataset) CurrentBalance() float64 {
	if d.accountsOK {
		if val, ok := helpers.CurrentBalanceFromAccounts(d.accounts); ok {
			return val
		}
	}
	snapshots := builders.BuildBalanceSnapshots(d.logs)
	if len(snapshots) == 0 {
		return 0
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})
	return snapshots[len(snapshots)-1].Balance
}

func (d *Dataset) Transactions() []domain.Transaction {
	transactions := builders.BuildTransactions(d.logs)
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
	fundings := builders.BuildFundings(d.logs, d.pairBySymbol)
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
