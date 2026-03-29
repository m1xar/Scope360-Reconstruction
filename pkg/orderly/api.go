package orderly

import (
	"crypto/ed25519"
	"errors"
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/domain"
	connector "github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly/executors"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/service/reconstructor"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/service/reconstructor/builders"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/service/reconstructor/envelope"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/service/reconstructor/helpers"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/service/reconstructor/workers"
)

const (
	defaultPositionWorkers = 8
	defaultCandleWorkers   = 4
)

type Config struct {
	BaseURL        string
	WalletAddress  string
	BrokerID       string
	Ed25519PubKey  string
	Ed25519PrivKey ed25519.PrivateKey
	HTTPClient     *resty.Client
}

func newClient(cfg Config) *connector.Client {
	creds := connector.NewCredentials(
		cfg.WalletAddress,
		cfg.BrokerID,
		cfg.Ed25519PubKey,
		cfg.Ed25519PrivKey,
	)

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = connector.MainnetBaseURL
	}

	return connector.NewClient(baseURL, creds, cfg.HTTPClient)
}

func GetBuiltPositions(cfg Config, days int) ([]domain.Position, error) {
	client := newClient(cfg)

	positions, err := reconstructClosedPositions(client, "")
	if err != nil {
		return nil, err
	}
	assetHistory, err := executors.FetchAssetHistory(client)
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
	cfg Config,
	pair string,
	openedAt time.Time,
	side string,
) (*domain.Position, error) {
	positions, err := GetBuiltPositions(cfg, 0)
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

func GetBalanceSnapshots(cfg Config, days int) ([]domain.UserBalanceSnapshot, error) {
	client := newClient(cfg)

	positions, err := reconstructClosedPositions(client, "")
	if err != nil {
		return nil, err
	}

	assetHistory, err := executors.FetchAssetHistory(client)
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

func GetCurrentBalance(cfg Config) (*float64, error) {
	snapshots, err := GetBalanceSnapshots(cfg, 0)
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, nil
	}

	balance := snapshots[len(snapshots)-1].Balance
	return &balance, nil
}

func reconstructClosedPositions(client *connector.Client, symbol string) ([]domain.Position, error) {
	trades, err := executors.FetchAllTrades(client, symbol, 0, 0)
	if err != nil {
		return nil, err
	}

	orders, err := executors.FetchFilledOrders(client, symbol, 0, 0)
	if err != nil {
		return nil, err
	}

	algoOrders, err := executors.FetchAlgoOrders(client, symbol, 0, 0)
	if err != nil {
		return nil, err
	}

	fundings, err := executors.FetchAllFunding(client, symbol, 0, 0)
	if err != nil {
		return nil, err
	}

	orderMap := helpers.BuildOrderMap(orders)
	algoIdx := helpers.BuildAlgoOrderIndex(algoOrders)

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, candleRequests, defaultCandleWorkers)

	envelopes := make(chan envelope.TradeEnvelope)
	positionsCh := make(chan domain.Position)

	go func() {
		reconstructor.ReconstructTrades(trades, fundings, orderMap, algoIdx, candleRequests, envelopes)
		close(envelopes)
		close(candleRequests)
	}()

	workers.StartPositionBuilders(envelopes, positionsCh, defaultPositionWorkers)

	positions := make([]domain.Position, 0)
	for pos := range positionsCh {
		positions = append(positions, pos)
	}

	sort.Slice(positions, func(i, j int) bool {
		iClosedAt := positions[i].ClosedAt
		jClosedAt := positions[j].ClosedAt
		if iClosedAt == nil && jClosedAt == nil {
			return i < j
		}
		if iClosedAt == nil {
			return false
		}
		if jClosedAt == nil {
			return true
		}
		return iClosedAt.Before(*jClosedAt)
	})

	return positions, nil
}

func GetFundings(cfg Config, days int) ([]domain.UserFunding, error) {
	client := newClient(cfg)

	var startTime int64
	if days > 0 {
		startTime = time.Now().AddDate(0, 0, -days).UnixMilli()
	}

	rawFundings, err := executors.FetchAllFunding(client, "", startTime, 0)
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
	cfg Config,
	coin string,
	interval string,
	startTime time.Time,
	endTime time.Time,
) ([]models.OrderlyCandle, error) {
	client := newClient(cfg)

	if endTime.Before(startTime) {
		return nil, errors.New("endTime must be >= startTime")
	}

	symbol := "PERP_" + coin + "_USDC"
	startMs := startTime.UnixMilli()
	endMs := endTime.UnixMilli()

	candles, err := executors.FetchCandles(client, symbol, interval, startMs, endMs)
	if err != nil {
		return nil, err
	}

	return candles, nil
}

func GetOpenPositions(cfg Config) ([]domain.OpenPosition, error) {
	client := newClient(cfg)

	positions, err := executors.FetchOpenPositions(client)
	if err != nil {
		return nil, err
	}

	return builders.BuildOpenPositions(positions), nil
}

func ValidateWalletSubscription(address, signature, message string) (bool, error) {
	ok := connector.VerifyWalletSignature(address, signature, message)
	return ok, nil
}
