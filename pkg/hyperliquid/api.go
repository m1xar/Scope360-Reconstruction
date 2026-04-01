package hyperliquid

import (
	"errors"
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/binance"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/executors"
	hlmodels "github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/workers"
)

const (
	defaultPositionWorkers = 8
	defaultCandleWorkers   = 4
	defaultTimeout         = 20 * time.Second
)

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

	fills, err := executors.FetchAllFills(client, endpoint, user)
	if err != nil {
		return nil, err
	}

	orders, err := executors.FetchHistoricalOrders(client, endpoint, user)
	if err != nil {
		return nil, err
	}

	rawFundings, err := executors.FetchAllFunding(client, endpoint, user, 0)
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

	orderIdx := helpers.BuildOrderIndex(orders)
	fills = helpers.NormalizeFills(fills)

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, endpoint, candleRequests, defaultCandleWorkers)

	envelopes := make(chan envelope.TradeEnvelope)
	positionsCh := make(chan domain.Position)

	go func() {
		reconstructor.ReconstructTrades(fills, rawFundings, orderIdx, candleRequests, envelopes)
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

	balanceSnapshots := builders.BuildUserBalanceSnapshotsFromPortfolio(portfolio)
	builders.ReconstructBalancesFromRawFills(fills, &balanceSnapshots)
	builders.AttachBalanceInitToPositions(&positions, balanceSnapshots)
	cutoff := helpers.CutoffFromDays(days)
	positions = helpers.FilterPositionsByClosedAt(positions, cutoff)
	for i := range positions {
		positions[i].Pair = helpers.NormalizeContractName(positions[i].Pair)
	}
	return positions, nil
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

	pair = helpers.NormalizeContractName(pair)
	coin := helpers.CoinFromPair(pair)

	allFills, err := executors.FetchAllFills(client, endpoint, user)
	if err != nil {
		return nil, err
	}

	fills := helpers.FilterFillsByCoinAndTime(helpers.NormalizeFills(allFills), coin, openedAt)

	orders, err := executors.FetchHistoricalOrders(client, endpoint, user)
	if err != nil {
		return nil, err
	}

	rawFundings, err := executors.FetchAllFunding(client, endpoint, user, 0)
	if err != nil {
		return nil, err
	}

	orderIdx := helpers.BuildOrderIndex(orders)

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, endpoint, candleRequests, defaultCandleWorkers)

	envelopes := make(chan envelope.TradeEnvelope)
	positionsCh := make(chan domain.Position)

	go func() {
		reconstructor.ReconstructTrades(fills, rawFundings, orderIdx, candleRequests, envelopes)
		close(envelopes)
		close(candleRequests)
	}()

	workers.StartPositionBuilders(envelopes, positionsCh, defaultPositionWorkers)

	var result *domain.Position
	for pos := range positionsCh {
		if result != nil {
			continue
		}
		pos.Pair = helpers.NormalizeContractName(pos.Pair)
		if pos.Pair == pair && pos.Side == side && pos.CreatedAt.Equal(openedAt) {
			matched := pos
			result = &matched
		}
	}

	if result != nil {
		rawPortfolio, err := executors.FetchPortfolioState(client, endpoint, user)
		if err != nil {
			return nil, err
		}
		portfolio, err := helpers.NormalizePortfolio(rawPortfolio)
		if err != nil {
			return nil, err
		}
		snapshots := builders.BuildUserBalanceSnapshotsFromPortfolio(portfolio)
		builders.ReconstructBalancesFromRawFills(allFills, &snapshots)
		positions := []domain.Position{*result}
		builders.AttachBalanceInitToPositions(&positions, snapshots)
		result = &positions[0]
	}

	return result, nil
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

	fills, err := executors.FetchAllFills(client, endpoint, user)
	if err != nil {
		return nil, err
	}

	fills = helpers.NormalizeFills(fills)

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
		return balanceSnapshots, nil
	}

	sort.Slice(balanceSnapshots, func(i, j int) bool {
		return balanceSnapshots[i].CreatedAt.Before(balanceSnapshots[j].CreatedAt)
	})

	firstFillMs := fills[0].Time
	for i := 1; i < len(fills); i++ {
		if fills[i].Time < firstFillMs {
			firstFillMs = fills[i].Time
		}
	}

	if firstFillMs >= balanceSnapshots[0].CreatedAt.UnixMilli() {
		return balanceSnapshots, nil
	}

	builders.ReconstructBalancesFromRawFills(fills, &balanceSnapshots)
	cutoff := helpers.CutoffFromDays(days)
	balanceSnapshots = helpers.FilterBalanceSnapshotsByCreatedAt(balanceSnapshots, cutoff)
	return balanceSnapshots, nil
}

func GetCurrentBalance(
	client *resty.Client,
	endpoint string,
	user string,
) (*float64, error) {
	balanceSnapshots, err := GetBalanceSnapshots(client, endpoint, user, 0)
	if err != nil {
		return nil, err
	}
	if len(balanceSnapshots) == 0 {
		return nil, nil
	}
	return &balanceSnapshots[len(balanceSnapshots)-1].Balance, nil
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

	rawFundings, err := executors.FetchAllFunding(client, endpoint, user, 0)
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
	if startMs < oldestAllowedMs {
		candles, err := binance.FetchFuturesKlinesPaged(
			client,
			coin,
			interval,
			startMs,
			endMs,
			499,
		)
		if err != nil {
			return nil, err
		}

		for i := range candles {
			candles[i].S = helpers.NormalizeContractName(candles[i].S)
		}
		return candles, nil
	}

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

	fills, err := executors.FetchAllFills(client, endpoint, user)
	if err != nil {
		return nil, err
	}

	fills = helpers.NormalizeFills(fills)

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, endpoint, candleRequests, defaultCandleWorkers)

	openPositions := builders.BuildOpenPositionsFromFills(candleRequests, fills)
	close(candleRequests)

	for i := range openPositions {
		openPositions[i].Pair = helpers.NormalizeContractName(openPositions[i].Pair)
	}
	return openPositions, nil
}

func ValidateWalletSubscription(address, signature, message string) (bool, error) {
	ok := helpers.VerifySignature(address, signature, message)
	return ok, nil
}
