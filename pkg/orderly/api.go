package orderly

import (
	"errors"
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	connector "github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly/models"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/service/reconstructor/helpers"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func newClient(httpClient *resty.Client, cfg connector.Config) *connector.Client {
	cfg.HTTPClient = httpClient
	return connector.NewClient(cfg)
}

func GetBuiltPositions(client *resty.Client, cfg connector.Config, days int) ([]domain.Position, error) {
	c := newClient(client, cfg)

	positions, err := reconstructor.ReconstructClosedPositions(c, "")
	if err != nil {
		return nil, err
	}
	if err := builders.EnrichPositionsWithCurrentRisk(c, &positions); err != nil {
		return nil, err
	}
	assetHistory, err := executors.FetchAssetHistory(c)
	if err != nil {
		return nil, err
	}
	balanceSnapshots := builders.BuildSyntheticBalanceSnapshotsFromStableTransfersAndClosedPositions(assetHistory, positions)
	builders.AttachBalanceInitToPositions(&positions, balanceSnapshots)

	cutoff := helpers.CutoffFromDays(days)
	positions = helpers.FilterPositionsByClosedAt(positions, cutoff)

	return positions, nil
}

func GetClosedPositionByExactMatch(
	client *resty.Client,
	cfg connector.Config,
	pair string,
	openedAt time.Time,
	side string,
) (*domain.Position, error) {
	positions, err := GetBuiltPositions(client, cfg, 0)
	if err != nil {
		return nil, err
	}

	normalizedPair := helpers.NormalizeSymbol(helpers.SymbolFromPair(pair))
	for _, pos := range positions {
		if pos.Pair == normalizedPair && pos.Side == side && pos.CreatedAt.Equal(openedAt) {
			matched := pos
			return &matched, nil
		}
	}

	return nil, nil
}

func GetBalanceSnapshots(client *resty.Client, cfg connector.Config, days int) ([]domain.UserBalanceSnapshot, error) {
	c := newClient(client, cfg)

	positions, err := reconstructor.ReconstructClosedPositions(c, "")
	if err != nil {
		return nil, err
	}

	assetHistory, err := executors.FetchAssetHistory(c)
	if err != nil {
		return nil, err
	}
	snapshots := builders.BuildSyntheticBalanceSnapshotsFromStableTransfersAndClosedPositions(assetHistory, positions)

	cutoff := helpers.CutoffFromDays(days)
	snapshots = helpers.FilterBalanceSnapshotsByCreatedAt(snapshots, cutoff)

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})

	return snapshots, nil
}

func GetCurrentBalance(client *resty.Client, cfg connector.Config) (*float64, error) {
	snapshots, err := GetBalanceSnapshots(client, cfg, 0)
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, nil
	}

	balance := snapshots[len(snapshots)-1].Balance
	return &balance, nil
}

func GetFundings(client *resty.Client, cfg connector.Config, days int) ([]domain.UserFunding, error) {
	c := newClient(client, cfg)

	var startTime int64
	if days > 0 {
		startTime = time.Now().AddDate(0, 0, -days).UnixMilli()
	}

	rawFundings, err := executors.FetchAllFunding(c, "", startTime, 0)
	if err != nil {
		return nil, err
	}

	fundings := make([]domain.UserFunding, 0, len(rawFundings))
	for _, f := range rawFundings {
		fundings = append(fundings, builders.BuildUserFunding(f))
	}

	cutoff := helpers.CutoffFromDays(days)
	fundings = helpers.FilterFundingsByCreatedAt(fundings, cutoff)

	return fundings, nil
}

func GetCandles(
	client *resty.Client,
	cfg connector.Config,
	coin string,
	interval string,
	startTime time.Time,
	endTime time.Time,
) ([]models.OrderlyCandle, error) {
	c := newClient(client, cfg)

	if endTime.Before(startTime) {
		return nil, errors.New("endTime must be >= startTime")
	}

	symbol := "PERP_" + coin + "_USDC"
	startMs := startTime.UnixMilli()
	endMs := endTime.UnixMilli()

	candles, err := executors.FetchCandles(c, symbol, interval, startMs, endMs)
	if err != nil {
		return nil, err
	}

	return candles, nil
}

func GetOpenPositions(client *resty.Client, cfg connector.Config) ([]domain.OpenPosition, error) {
	c := newClient(client, cfg)

	positions, err := executors.FetchOpenPositions(c)
	if err != nil {
		return nil, err
	}

	return builders.BuildOpenPositions(positions), nil
}

func ValidateWalletSubscription(address, signature, message string) (bool, error) {
	ok := connector.VerifyWalletSignature(address, signature, message)
	return ok, nil
}
