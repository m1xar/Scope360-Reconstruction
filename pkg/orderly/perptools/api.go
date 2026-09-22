package perptools

import (
	"errors"
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	connector "github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/models"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/reconstructor/helpers"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func newClient(httpClient *resty.Client, cfg connector.Config) *connector.Client {
	cfg.HTTPClient = httpClient
	return connector.NewClient(cfg)
}

func GetBuiltPositions(client *resty.Client, cfg connector.Config, days int) ([]domain.Position, error) {
	c := newClient(client, cfg)

	cutoff := helpers.CutoffFromDays(days)

	positions, err := reconstructor.ReconstructClosedPositions(c, "", cutoff)
	if err != nil {
		return nil, err
	}
	if err := reconstructor.EnrichPositionsWithCurrentRisk(c, &positions); err != nil {
		return nil, err
	}

	snapshots, err := reconstructor.BalanceSnapshots(c, positions, cutoff)
	if err != nil {
		return nil, err
	}
	helpers.AttachBalanceInit(&positions, snapshots)

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

	cutoff := helpers.CutoffFromDays(days)

	positions, err := reconstructor.ReconstructClosedPositions(c, "", cutoff)
	if err != nil {
		return nil, err
	}

	snapshots, err := reconstructor.BalanceSnapshots(c, positions, cutoff)
	if err != nil {
		return nil, err
	}

	snapshots = helpers.FilterBalanceSnapshotsByCreatedAt(snapshots, cutoff)

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})

	return snapshots, nil
}

func GetCurrentBalance(client *resty.Client, cfg connector.Config) (*float64, error) {
	c := newClient(client, cfg)

	snapshot, err := executors.FetchPositionsSnapshot(c)
	if err != nil {
		return nil, err
	}

	balance := helpers.Round8(snapshot.AccountValue)
	return &balance, nil
}

func GetTransactions(client *resty.Client, cfg connector.Config, days int) ([]domain.Transaction, error) {
	c := newClient(client, cfg)

	cutoff := helpers.CutoffFromDays(days)
	startMs := int64(0)
	if cutoff != nil {
		startMs = cutoff.UnixMilli()
	}
	assetHistory, err := executors.FetchAssetHistory(c, startMs, 0)
	if err != nil {
		return nil, err
	}

	markPrices, err := executors.FetchMarkPrices(c)
	if err != nil {
		return nil, err
	}

	transactions, err := builders.BuildTransactions(assetHistory, markPrices)
	if err != nil {
		return nil, err
	}
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

	rawPositions, err := executors.FetchOpenPositions(c)
	if err != nil {
		return nil, err
	}

	positions := builders.BuildOpenPositions(rawPositions)
	reconstructor.EnrichOpenPositionOrders(c, positions)
	return positions, nil
}

func ValidateWalletSubscription(address, signature, message string) (bool, error) {
	ok := connector.VerifyWalletSignature(address, signature, message)
	return ok, nil
}
