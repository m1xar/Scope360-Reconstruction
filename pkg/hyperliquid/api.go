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
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

const defaultTimeout = 20 * time.Second

func newDefaultClient() *resty.Client {
	return resty.New().SetTimeout(defaultTimeout)
}

func GetBuiltPositions(
	client *resty.Client,
	endpoint string,
	user string,
	days int,
) ([]domain.Position, error) {
	if client == nil {
		client = newDefaultClient()
	}

	return reconstructor.ReconstructClosedPositions(client, endpoint, user, helpers.CutoffFromDays(days))
}

func GetBalanceSnapshots(
	client *resty.Client,
	endpoint string,
	user string,
	days int,
) ([]domain.UserBalanceSnapshot, error) {
	if client == nil {
		client = newDefaultClient()
	}

	cutoff := helpers.CutoffFromDays(days)

	fills, err := reconstructor.FillsSince(client, endpoint, user, cutoff)
	if err != nil {
		return nil, err
	}

	rawPortfolio, err := executors.FetchPortfolioState(client, endpoint, user)
	if err != nil {
		return nil, err
	}

	portfolio, err := helpers.NormalizePortfolio(rawPortfolio)
	if err != nil {
		return nil, err
	}

	balanceSnapshots := builders.BuildUserBalanceSnapshotsFromPortfolio(portfolio)
	if len(balanceSnapshots) == 0 || len(fills) == 0 {
		return helpers.FilterBalanceSnapshotsByCreatedAt(balanceSnapshots, cutoff), nil
	}

	sort.Slice(balanceSnapshots, func(i, j int) bool {
		return balanceSnapshots[i].CreatedAt.Before(balanceSnapshots[j].CreatedAt)
	})

	helpers.ReconstructBalancesFromRawFills(fills, &balanceSnapshots)
	balanceSnapshots = helpers.FilterBalanceSnapshotsByCreatedAt(balanceSnapshots, cutoff)
	return balanceSnapshots, nil
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
	if client == nil {
		client = newDefaultClient()
	}

	startMs := int64(0)
	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		startMs = cutoff.UnixMilli()
	}

	updates, err := executors.FetchAllNonFundingLedgerUpdates(client, endpoint, user, startMs, 0)
	if err != nil {
		return nil, err
	}

	transactions := builders.BuildTransactions(updates)
	if cutoff != nil {
		filtered := transactions[:0]
		for _, tx := range transactions {
			if !tx.Time.Before(*cutoff) {
				filtered = append(filtered, tx)
			}
		}
		transactions = filtered
	}
	return transactions, nil
}

func GetFundings(
	client *resty.Client,
	endpoint string,
	user string,
	days int,
) ([]domain.UserFunding, error) {
	if client == nil {
		client = newDefaultClient()
	}

	rawFundings, err := executors.FetchAllFunding(client, endpoint, user, window.StartMs(helpers.CutoffFromDays(days)))
	if err != nil {
		return nil, err
	}

	fundings := make([]domain.UserFunding, 0, len(rawFundings))
	for _, fund := range rawFundings {
		fundings = append(fundings, builders.BuildUserFunding(fund))
	}

	cutoff := helpers.CutoffFromDays(days)
	fundings = helpers.FilterFundingsByCreatedAt(fundings, cutoff)
	for i := range fundings {
		fundings[i].Pair = helpers.NormalizeContractName(fundings[i].Pair)
	}
	return fundings, nil
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
	if client == nil {
		client = newDefaultClient()
	}
	_ = days

	return reconstructor.ReconstructOpenPositions(client, endpoint, user)
}
