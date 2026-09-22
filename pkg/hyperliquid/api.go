package hyperliquid

import (
	"errors"
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/binance"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/executors"
	hlmodels "github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
)

const defaultTimeout = 20 * time.Second

func newDefaultClient() *resty.Client {
	return resty.New().SetTimeout(defaultTimeout)
}

func load(client *resty.Client, endpoint, user string, days int, s scope.Scope) (*reconstructor.Dataset, error) {
	if client == nil {
		client = newDefaultClient()
	}
	return reconstructor.Load(client, endpoint, user, helpers.CutoffFromDays(days), s)
}

// Sync fetches the account's raw data once and builds every model from it:
// closed and open positions, balance snapshots, the current balance,
// transactions and fundings for the last days (the whole history when
// days <= 0). Open positions need the whole fill history, so Sync loads it
// once and serves the closed positions from it too.
func Sync(
	client *resty.Client,
	endpoint string,
	user string,
	days int,
) (*domain.Sync, error) {
	d, err := load(client, endpoint, user, days, scope.All)
	if err != nil {
		return nil, err
	}

	out := &domain.Sync{}
	err = parallel.Run(
		func() (err error) { out.Positions, err = d.ClosedPositions(); return err },
		func() (err error) { out.OpenPositions, err = d.OpenPositions(); return err },
		func() error { out.BalanceSnapshots = d.BalanceSnapshots(); return nil },
		func() error { out.Transactions = d.Transactions(); return nil },
		func() error { out.Fundings = d.Fundings(); return nil },
	)
	if err != nil {
		return nil, err
	}
	out.CurrentBalance = d.CurrentBalance()
	return out, nil
}

func GetBuiltPositions(
	client *resty.Client,
	endpoint string,
	user string,
	days int,
) ([]domain.Position, error) {
	d, err := load(client, endpoint, user, days, scope.Closed)
	if err != nil {
		return nil, err
	}
	return d.ClosedPositions()
}

func GetBalanceSnapshots(
	client *resty.Client,
	endpoint string,
	user string,
	days int,
) ([]domain.UserBalanceSnapshot, error) {
	d, err := load(client, endpoint, user, days, scope.Balances)
	if err != nil {
		return nil, err
	}
	return d.BalanceSnapshots(), nil
}

func GetCurrentBalance(
	client *resty.Client,
	endpoint string,
	user string,
) (*float64, error) {
	if client == nil {
		client = newDefaultClient()
	}

	rawPortfolio, err := executors.FetchPortfolioState(client, endpoint, user)
	if err != nil {
		return nil, err
	}

	portfolio, err := helpers.NormalizePortfolio(rawPortfolio)
	if err != nil {
		return nil, err
	}

	snapshots := builders.BuildUserBalanceSnapshotsFromPortfolio(portfolio)
	if len(snapshots) == 0 {
		return nil, nil
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})
	return &snapshots[len(snapshots)-1].Balance, nil
}

func GetTransactions(
	client *resty.Client,
	endpoint string,
	user string,
	days int,
) ([]domain.Transaction, error) {
	d, err := load(client, endpoint, user, days, scope.Transactions)
	if err != nil {
		return nil, err
	}
	return d.Transactions(), nil
}

func GetFundings(
	client *resty.Client,
	endpoint string,
	user string,
	days int,
) ([]domain.UserFunding, error) {
	d, err := load(client, endpoint, user, days, scope.Fundings)
	if err != nil {
		return nil, err
	}
	return d.Fundings(), nil
}

func GetCandles(
	client *resty.Client,
	endpoint string,
	coin string,
	interval string,
	startTime time.Time,
	endTime time.Time,
) ([]hlmodels.HyperliquidCandle, error) {
	if client == nil {
		client = newDefaultClient()
	}

	if endTime.Before(startTime) {
		return nil, errors.New("endTime must be >= startTime")
	}

	intervalMs, err := helpers.IntervalToMs(interval)
	if err != nil {
		return nil, err
	}
	startMs := startTime.UnixMilli()
	endMs := endTime.UnixMilli()

	oldestAllowedMs := time.Now().UnixMilli() - intervalMs*5000
	if startMs >= oldestAllowedMs {
		candles, err := executors.FetchAllCandlesHyperliquid(
			client,
			endpoint,
			coin,
			interval,
			startMs,
			endMs,
		)
		if err != nil {
			return nil, err
		}
		for i := range candles {
			candles[i].S = helpers.NormalizeContractName(candles[i].S)
		}
		return candles, nil
	}

	var out []hlmodels.HyperliquidCandle
	binanceEnd := endMs
	if binanceEnd > oldestAllowedMs {
		binanceEnd = oldestAllowedMs - 1
	}
	if binanceEnd >= startMs {
		candles, err := binance.FetchFuturesKlinesPaged(
			client,
			coin,
			interval,
			startMs,
			binanceEnd,
			499,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, candles...)
	}
	if endMs >= oldestAllowedMs {
		hlStart := oldestAllowedMs
		if hlStart < startMs {
			hlStart = startMs
		}
		candles, err := executors.FetchAllCandlesHyperliquid(
			client,
			endpoint,
			coin,
			interval,
			hlStart,
			endMs,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, candles...)
	}
	for i := range out {
		out[i].S = helpers.NormalizeContractName(out[i].S)
	}
	return out, nil
}

func ValidateWalletSubscription(address, signature, message string) (bool, error) {
	ok := helpers.VerifySignature(address, signature, message)
	return ok, nil
}

func GetClosedPositionByExactMatch(
	client *resty.Client,
	endpoint string,
	user string,
	pair string,
	openedAt time.Time,
	side string,
) (*domain.Position, error) {
	if client == nil {
		client = newDefaultClient()
	}

	return reconstructor.FindClosedPosition(client, endpoint, user, pair, openedAt, side)
}

func GetOpenPositions(
	client *resty.Client,
	endpoint string,
	user string,
	days int,
) ([]domain.OpenPosition, error) {
	_ = days // open positions need the whole fill history

	d, err := load(client, endpoint, user, 0, scope.Open)
	if err != nil {
		return nil, err
	}
	return d.OpenPositions()
}
