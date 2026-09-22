package reconstructor

import (
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/workers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

type Dataset struct {
	client         *resty.Client
	endpoint, user string
	cutoff         *time.Time
	scope          scope.Scope

	fills    []models.RawFill
	segments [][]models.RawFill

	orders      []models.HistoricalOrder
	fundings    []models.FundingHistoryItem
	portfolio   models.PortfolioResponse
	portfolioOK bool
	ledger      []models.NonFundingLedgerUpdate
}

func Load(client *resty.Client, endpoint, user string, cutoff *time.Time, s scope.Scope) (*Dataset, error) {
	d := &Dataset{client: client, endpoint: endpoint, user: user, cutoff: cutoff, scope: s}
	cutoffMs := window.StartMs(cutoff)

	err := parallel.Run(
		when(s.Any(scope.Closed|scope.Open|scope.Balances), func() error {
			return d.loadFills()
		}),
		when(s.Has(scope.Closed), func() error {
			orders, err := executors.FetchHistoricalOrders(client, endpoint, user)
			if err != nil {
				return err
			}
			d.orders = orders
			return nil
		}),
		when(s.Any(scope.Closed|scope.Balances), func() error {
			raw, err := executors.FetchPortfolioState(client, endpoint, user)
			if err != nil {
				return err
			}
			portfolio, err := helpers.NormalizePortfolio(raw)
			if err != nil {
				return err
			}
			d.portfolio, d.portfolioOK = portfolio, true
			return nil
		}),
		when(s.Has(scope.Transactions), func() error {
			updates, err := executors.FetchAllNonFundingLedgerUpdates(client, endpoint, user, cutoffMs, 0)
			if err != nil {
				return err
			}
			d.ledger = updates
			return nil
		}),
		when(s.Has(scope.Fundings) && !s.Has(scope.Closed), func() error {
			return d.loadFundings(cutoffMs)
		}),
	)
	if err != nil {
		return nil, err
	}

	if s.Has(scope.Closed) && (len(d.segments) > 0 || s.Has(scope.Fundings)) {
		since := int64(0)
		if len(d.segments) > 0 {
			since = d.segments[0][0].Time
			for _, seg := range d.segments {
				if seg[0].Time < since {
					since = seg[0].Time
				}
			}
		}
		if s.Has(scope.Fundings) && (cutoff == nil || cutoffMs < since) {
			since = cutoffMs
		}
		if err := d.loadFundings(since); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func when(cond bool, fn func() error) func() error {
	if !cond {
		return nil
	}
	return fn
}

func (d *Dataset) loadFills() error {
	switch {
	case d.scope.Has(scope.Open) || (d.scope.Has(scope.Closed) && d.cutoff == nil):
		fills, err := executors.FetchAllFills(d.client, d.endpoint, d.user)
		if err != nil {
			return err
		}
		d.fills = helpers.NormalizeFills(fills)
		if d.scope.Has(scope.Closed) {
			d.segments = helpers.EpisodesClosedAfter(helpers.SegmentFills(d.fills), d.cutoff)
		}
	case d.scope.Has(scope.Closed):
		walk, err := CollectEpisodes(d.client, d.endpoint, d.user, d.cutoff)
		if err != nil {
			return err
		}
		d.fills = walk.Fills
		d.segments = helpers.EpisodesClosedAfter(walk.Segments, d.cutoff)
	default:
		fills, err := FillsSince(d.client, d.endpoint, d.user, d.cutoff)
		if err != nil {
			return err
		}
		d.fills = fills
	}
	return nil
}

func (d *Dataset) loadFundings(sinceMs int64) error {
	fundings, err := executors.FetchAllFunding(d.client, d.endpoint, d.user, sinceMs)
	if err != nil {
		return err
	}
	d.fundings = fundings
	return nil
}

func (d *Dataset) fillsFrom(fromMs int64) []models.RawFill {
	if fromMs <= 0 {
		return append([]models.RawFill(nil), d.fills...)
	}
	out := make([]models.RawFill, 0, len(d.fills))
	for _, f := range d.fills {
		if f.Time >= fromMs {
			out = append(out, f)
		}
	}
	return out
}

func (d *Dataset) snapshotsFrom(fromMs int64) []domain.UserBalanceSnapshot {
	snapshots := builders.BuildUserBalanceSnapshotsFromPortfolio(d.portfolio)
	fills := d.fillsFrom(fromMs)
	if len(snapshots) == 0 || len(fills) == 0 {
		return snapshots
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})
	helpers.ReconstructBalancesFromRawFills(fills, &snapshots)
	return snapshots
}

func (d *Dataset) ClosedPositions() ([]domain.Position, error) {
	if len(d.segments) == 0 {
		return []domain.Position{}, nil
	}

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(d.client, d.endpoint, candleRequests, defaultCandleWorkers)

	envelopes := make(chan envelope.TradeEnvelope)
	positionsCh := make(chan domain.Position)

	go func() {
		ReconstructTrades(d.segments, d.fundings, helpers.BuildOrderIndex(d.orders), candleRequests, envelopes)
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

	earliest := d.segments[0][0].Time
	for _, seg := range d.segments {
		if seg[0].Time < earliest {
			earliest = seg[0].Time
		}
	}
	if cutoffMs := window.StartMs(d.cutoff); cutoffMs > 0 && cutoffMs < earliest {
		earliest = cutoffMs
	}
	helpers.AttachBalanceInit(&positions, d.snapshotsFrom(earliest))
	positions = helpers.FilterPositionsByClosedAt(positions, d.cutoff)
	for i := range positions {
		positions[i].Pair = helpers.NormalizeContractName(positions[i].Pair)
	}
	return positions, nil
}

func (d *Dataset) OpenPositions() ([]domain.OpenPosition, error) {
	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(d.client, d.endpoint, candleRequests, defaultCandleWorkers)

	openPositions := builders.BuildOpenPositionsFromFills(candleRequests, append([]models.RawFill(nil), d.fills...))
	close(candleRequests)

	for i := range openPositions {
		openPositions[i].Pair = helpers.NormalizeContractName(openPositions[i].Pair)
	}
	return openPositions, nil
}

func (d *Dataset) BalanceSnapshots() []domain.UserBalanceSnapshot {
	return helpers.FilterBalanceSnapshotsByCreatedAt(d.snapshotsFrom(window.StartMs(d.cutoff)), d.cutoff)
}

func (d *Dataset) CurrentBalance() float64 {
	snapshots := builders.BuildUserBalanceSnapshotsFromPortfolio(d.portfolio)
	if len(snapshots) == 0 {
		return 0
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})
	return snapshots[len(snapshots)-1].Balance
}

func (d *Dataset) Transactions() []domain.Transaction {
	transactions := builders.BuildTransactions(d.ledger)
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
	fundings := make([]domain.UserFunding, 0, len(d.fundings))
	for _, fund := range d.fundings {
		fundings = append(fundings, builders.BuildUserFunding(fund))
	}
	fundings = helpers.FilterFundingsByCreatedAt(fundings, d.cutoff)
	for i := range fundings {
		fundings[i].Pair = helpers.NormalizeContractName(fundings[i].Pair)
	}
	return fundings
}
