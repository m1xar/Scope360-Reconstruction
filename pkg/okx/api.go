package okx

import (
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	okxclient "github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

func GetAuthStatus(apiKey, secret, passphrase string) (string, okxclient.Region) {
	region, err := okxclient.CheckAccount(apiKey, secret, passphrase)
	if err != nil {
		return "error", ""
	}

	return "ok", region
}

func GetClosedPositionByExactMatch(
	client *resty.Client,
	creds okxclient.Credentials,
	baseURL string,
	pair string,
	openedAt time.Time,
	side string,
) (*domain.Position, error) {
	positions, err := GetBuiltPositions(client, creds, baseURL, window.DaysSince(openedAt))
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

func GetBalanceSnapshots(
	client *resty.Client,
	creds okxclient.Credentials,
	baseURL string,
	days int,
) ([]domain.UserBalanceSnapshot, error) {
	okxclient.AttachAuth(client, creds)

	balance, err := executors.FetchBalance(client, baseURL)
	if err != nil {
		return nil, err
	}
	currentBal := helpers.MustFloat(balance.TotalEq)

	startMs := int64(0)
	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		startMs = cutoff.UnixMilli()
	}

	bills, err := executors.FetchAllSwapAndFuturesBills(client, baseURL, startMs, "")
	if err != nil {
		return nil, err
	}
	snapshots := builders.BuildBalanceSnapshotsFromBills(currentBal, bills)
	if cutoff != nil {
		filtered := snapshots[:0]
		for _, s := range snapshots {
			if !s.CreatedAt.Before(*cutoff) {
				filtered = append(filtered, s)
			}
		}
		snapshots = filtered
	}

	return snapshots, nil
}

func GetCurrentBalance(
	client *resty.Client,
	creds okxclient.Credentials,
	baseURL string,
) (*float64, error) {
	okxclient.AttachAuth(client, creds)

	balance, err := executors.FetchBalance(client, baseURL)
	if err != nil {
		return nil, err
	}

	val := helpers.MustFloat(balance.TotalEq)
	return &val, nil
}

func GetTransactions(
	client *resty.Client,
	creds okxclient.Credentials,
	baseURL string,
	days int,
) ([]domain.Transaction, error) {
	okxclient.AttachAuth(client, creds)

	startMs := int64(0)
	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		startMs = cutoff.UnixMilli()
	}

	bills, err := executors.FetchAllBills(client, baseURL, "", startMs, "")
	if err != nil {
		return nil, err
	}

	transactions := builders.BuildTransactionsFromBills(bills)
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
	creds okxclient.Credentials,
	baseURL string,
	days int,
) ([]domain.UserFunding, error) {
	okxclient.AttachAuth(client, creds)

	startMs := int64(0)
	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		startMs = cutoff.UnixMilli()
	}

	bills, err := executors.FetchAllSwapAndFuturesBills(client, baseURL, startMs, "8")
	if err != nil {
		return nil, err
	}

	fundings := make([]domain.UserFunding, 0, len(bills))
	for _, b := range bills {
		amount := helpers.MustFloat(b.BalChg)
		if amount == 0 {
			continue
		}
		fundings = append(fundings, domain.UserFunding{
			Pair:      helpers.NormalizePair(b.InstId),
			Amount:    helpers.Round8(amount),
			CreatedAt: helpers.TimeFromMs(b.Ts),
		})
	}

	return fundings, nil
}

func GetCandles(
	client *resty.Client,
	baseURL string,
	instId string,
	bar string,
	startTime time.Time,
	endTime time.Time,
) ([]models.Candle, error) {
	if client == nil {
		client = okxclient.NewBaseClient()
	}

	if endTime.Before(startTime) {
		return nil, fmt.Errorf("endTime must be >= startTime")
	}

	return executors.FetchCandles(
		client, baseURL, instId, bar,
		startTime.UnixMilli(), endTime.UnixMilli(),
	)
}

func GetBuiltPositions(
	client *resty.Client,
	creds okxclient.Credentials,
	baseURL string,
	days int,
) ([]domain.Position, error) {
	okxclient.AttachAuth(client, creds)

	return reconstructor.ReconstructClosedPositions(client, baseURL, days)
}

func GetOpenPositions(
	client *resty.Client,
	creds okxclient.Credentials,
	baseURL string,
) ([]domain.OpenPosition, error) {
	okxclient.AttachAuth(client, creds)

	return reconstructor.ReconstructOpenPositions(client, baseURL)
}
