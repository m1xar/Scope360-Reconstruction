package blofin

import (
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	blofinclient "github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

const (
	defaultCandleWorkers   = 4
	defaultPositionWorkers = 8
)

func GetAuthStatus(apiKey, secret, passphrase string) string {
	if err := blofinclient.CheckAccount(apiKey, secret, passphrase); err != nil {
		return "error"
	}

	return "ok"
}

func GetBuiltPositions(
	client *resty.Client,
	creds blofinclient.Credentials,
	days int,
) ([]domain.Position, error) {
	blofinclient.AttachAuth(client, creds)

	return reconstructor.ReconstructClosedPositions(client, blofinclient.BaseURL, helpers.CutoffFromDays(days))
}

func GetClosedPositionByExactMatch(
	client *resty.Client,
	creds blofinclient.Credentials,
	pair string,
	openedAt time.Time,
	side string,
) (*domain.Position, error) {
	positions, err := GetBuiltPositions(client, creds, window.DaysSince(openedAt))
	if err != nil {
		return nil, err
	}

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
	creds blofinclient.Credentials,
) ([]domain.OpenPosition, error) {
	blofinclient.AttachAuth(client, creds)

	return reconstructor.ReconstructOpenPositions(client, blofinclient.BaseURL)
}

func GetBalanceSnapshots(
	client *resty.Client,
	creds blofinclient.Credentials,
	days int,
) ([]domain.UserBalanceSnapshot, error) {

	cutoff := helpers.CutoffFromDays(days)

	positions, err := GetBuiltPositions(client, creds, days)
	if err != nil {
		return nil, err
	}

	snapshots, err := reconstructor.BalanceSnapshots(client, blofinclient.BaseURL, positions, cutoff)
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
	creds blofinclient.Credentials,
) (*float64, error) {
	blofinclient.AttachAuth(client, creds)

	equity, err := executors.FetchTotalEquity(client, blofinclient.BaseURL)
	if err != nil {
		return nil, err
	}

	return &equity, nil
}

func GetTransactions(
	client *resty.Client,
	creds blofinclient.Credentials,
	days int,
) ([]domain.Transaction, error) {
	blofinclient.AttachAuth(client, creds)

	startMs := int64(0)
	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		startMs = cutoff.UnixMilli()
	}

	transfers, err := executors.FetchAllTransfers(client, blofinclient.BaseURL, startMs)
	if err != nil {
		return nil, err
	}

	transactions := builders.BuildTransactionsFromTransfers(transfers)
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
	creds blofinclient.Credentials,
	days int,
) ([]domain.UserFunding, error) {
	blofinclient.AttachAuth(client, creds)

	startMs := int64(0)
	if cutoff := helpers.CutoffFromDays(days); cutoff != nil {
		startMs = cutoff.UnixMilli()
	}

	fees, err := executors.FetchAllFundingFees(client, blofinclient.BaseURL, startMs)
	if err != nil {
		return nil, err
	}

	return builders.BuildFundings(fees), nil
}

func GetCandles(
	client *resty.Client,
	instID string,
	bar string,
	startTime time.Time,
	endTime time.Time,
) ([]models.Candle, error) {
	if client == nil {
		client = blofinclient.NewBaseClient()
	}

	if endTime.Before(startTime) {
		return nil, fmt.Errorf("endTime must be >= startTime")
	}

	return executors.FetchCandles(
		client, blofinclient.BaseURL, instID, bar,
		startTime.UnixMilli(), endTime.UnixMilli(),
	)
}
