package hyperliquid

import (
	"net/http"
	"sort"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/executors"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/builders"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/workers"
)

const defaultPositionWorkers = 8

func GetBuiltPositions(
	client *http.Client,
	endpoint string,
	user string,
	days int,
) ([]domain.Position, error) {
	if client == nil {
		client = http.DefaultClient
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

	envelopes := make(chan models.TradeEnvelope)
	positionsCh := make(chan domain.Position)

	go func() {
		reconstructor.ReconstructTrades(fills, rawFundings, orderIdx, client, endpoint, envelopes)
		close(envelopes)
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
	if cutoff != nil {
		filtered := positions[:0]
		for _, pos := range positions {
			if !pos.ClosedAt.Before(*cutoff) {
				filtered = append(filtered, pos)
			}
		}
		positions = filtered
	}
	return positions, nil
}

func GetBalanceSnapshots(
	client *http.Client,
	endpoint string,
	user string,
	days int,
) ([]domain.UserBalanceSnapshot, error) {
	if client == nil {
		client = http.DefaultClient
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
	if cutoff != nil {
		filtered := balanceSnapshots[:0]
		for _, snapshot := range balanceSnapshots {
			if !snapshot.CreatedAt.Before(*cutoff) {
				filtered = append(filtered, snapshot)
			}
		}
		balanceSnapshots = filtered
	}
	return balanceSnapshots, nil
}

func GetCurrentBalance(
	client *http.Client,
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
	client *http.Client,
	endpoint string,
	user string,
	days int,
) ([]domain.UserFunding, error) {
	if client == nil {
		client = http.DefaultClient
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
	if cutoff != nil {
		filtered := fundings[:0]
		for _, funding := range fundings {
			if !funding.CreatedAt.Before(*cutoff) {
				filtered = append(filtered, funding)
			}
		}
		fundings = filtered
	}
	return fundings, nil
}

func GetOpenPositions(
	client *http.Client,
	endpoint string,
	user string,
	days int,
) ([]domain.OpenPosition, error) {
	if client == nil {
		client = http.DefaultClient
	}
	_ = days

	state, err := executors.FetchClearinghouseState(client, endpoint, user)
	if err != nil {
		return nil, err
	}

	return builders.BuildOpenPositionsFromClearinghouse(state, client, endpoint)
}

func ValidateWalletSubscription(address, signature, message string) (bool, error) {
	ok := helpers.VerifySignature(address, signature, message)
	return ok, nil
}
