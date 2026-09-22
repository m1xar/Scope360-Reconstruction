package bybit

import (
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	bybitclient "github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit/models"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

func authClient(client *resty.Client, creds bybitclient.Credentials) *resty.Client {
	if client == nil {
		client = bybitclient.NewBaseClient()
	}
	bybitclient.AttachAuth(client, creds)
	return client
}

func GetAuthStatus(apiKey, secret string) string {
	if err := bybitclient.CheckAccount(apiKey, secret); err != nil {
		return "error"
	}
	return "ok"
}

func GetBuiltPositions(
	client *resty.Client,
	creds bybitclient.Credentials,
	days int,
) ([]domain.Position, error) {
	client = authClient(client, creds)

	return reconstructor.ReconstructClosedPositions(client, helpers.CutoffFromDays(days))
}

func GetClosedPositionByExactMatch(
	client *resty.Client,
	creds bybitclient.Credentials,
	pair string,
	openedAt time.Time,
	side string,
) (*domain.Position, error) {
	positions, err := GetBuiltPositions(client, creds, window.DaysSince(openedAt))
	if err != nil {
		return nil, err
	}

	pair = helpers.NormalizePair(pair)
	for i := range positions {
		pos := &positions[i]
		if pos.Pair == pair && pos.Side == side && pos.CreatedAt.Equal(openedAt) {
			return pos, nil
		}
	}
	return nil, nil
}

func GetOpenPositions(
	client *resty.Client,
	creds bybitclient.Credentials,
) ([]domain.OpenPosition, error) {
	client = authClient(client, creds)

	return reconstructor.ReconstructOpenPositions(client)
}

func GetBalanceSnapshots(
	client *resty.Client,
	creds bybitclient.Credentials,
	days int,
) ([]domain.UserBalanceSnapshot, error) {
	client = authClient(client, creds)

	cutoff := helpers.CutoffFromDays(days)

	snapshots, err := reconstructor.BalanceSnapshots(client, nil, cutoff)
	if err != nil {
		return nil, err
	}
	if cutoff == nil {
		return snapshots, nil
	}

	filtered := snapshots[:0]
	for _, s := range snapshots {
		if !s.CreatedAt.Before(*cutoff) {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

func GetCurrentBalance(
	client *resty.Client,
	creds bybitclient.Credentials,
) (*float64, error) {
	client = authClient(client, creds)

	wallet, err := executors.FetchWalletBalance(client)
	if err != nil {
		return nil, err
	}

	equity := helpers.Round8(executors.TotalEquity(wallet))
	return &equity, nil
}

func GetTransactions(
	client *resty.Client,
	creds bybitclient.Credentials,
	days int,
) ([]domain.Transaction, error) {
	client = authClient(client, creds)

	startMs := int64(0)
	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		startMs = cutoff.UnixMilli()
	}

	var rows []models.LedgerEntry
	for _, typ := range []string{models.LedgerTransferIn, models.LedgerTransferOut} {
		page, err := executors.FetchLedger(client, startMs, 0, executors.LedgerFilter{Type: typ})
		if err != nil {
			return nil, err
		}
		rows = append(rows, page...)
	}

	transactions := builders.BuildTransactions(helpers.BuildLedger(rows))
	if cutoff == nil {
		return transactions, nil
	}

	filtered := transactions[:0]
	for _, tx := range transactions {
		if !tx.Time.Before(*cutoff) {
			filtered = append(filtered, tx)
		}
	}
	return filtered, nil
}

func GetFundings(
	client *resty.Client,
	creds bybitclient.Credentials,
	days int,
) ([]domain.UserFunding, error) {
	client = authClient(client, creds)

	startMs := int64(0)
	if cutoff := helpers.CutoffFromDays(days); cutoff != nil {
		startMs = cutoff.UnixMilli()
	}

	rows, err := executors.FetchLedger(client, startMs, 0, executors.LedgerFilter{
		Category: models.CategoryLinear,
		Type:     models.LedgerSettlement,
	})
	if err != nil {
		return nil, err
	}

	return builders.BuildFundings(helpers.BuildLedger(rows)), nil
}

func GetCandles(
	client *resty.Client,
	symbol string,
	interval string,
	startTime time.Time,
	endTime time.Time,
) ([]models.Candle, error) {
	if client == nil {
		client = bybitclient.NewBaseClient()
	}

	if endTime.Before(startTime) {
		return nil, fmt.Errorf("endTime must be >= startTime")
	}

	return executors.FetchCandles(
		client, symbol, interval,
		startTime.UnixMilli(), endTime.UnixMilli(),
	)
}
