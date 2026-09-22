package kraken

import (
	"fmt"
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	krakenclient "github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

func authClient(client *resty.Client, creds krakenclient.Credentials) *resty.Client {
	if client == nil {
		return krakenclient.NewClient(creds)
	}
	krakenclient.AttachAuth(client, creds)
	return client
}

func GetAuthStatus(client *resty.Client, creds krakenclient.Credentials) string {
	client = authClient(client, creds)
	if _, err := executors.CheckAPIKey(client); err != nil {
		return "error"
	}
	return "ok"
}

func load(client *resty.Client, creds krakenclient.Credentials, days int, s scope.Scope) (*reconstructor.Dataset, error) {
	return reconstructor.Load(authClient(client, creds), helpers.CutoffFromDays(days), s)
}

func Sync(
	client *resty.Client,
	creds krakenclient.Credentials,
	days int,
) (*domain.Sync, error) {
	d, err := load(client, creds, days, scope.All)
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
	creds krakenclient.Credentials,
	days int,
) ([]domain.Position, error) {
	d, err := load(client, creds, days, scope.Closed)
	if err != nil {
		return nil, err
	}
	return d.ClosedPositions()
}

func GetClosedPositionByExactMatch(
	client *resty.Client,
	creds krakenclient.Credentials,
	pair string,
	openedAt time.Time,
	side string,
) (*domain.Position, error) {
	positions, err := GetBuiltPositions(client, creds, window.DaysSince(openedAt))
	if err != nil {
		return nil, err
	}

	pair = helpers.NormalizePairText(pair)
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
	creds krakenclient.Credentials,
) ([]domain.OpenPosition, error) {
	d, err := load(client, creds, 0, scope.Open)
	if err != nil {
		return nil, err
	}
	return d.OpenPositions()
}

func GetBalanceSnapshots(
	client *resty.Client,
	creds krakenclient.Credentials,
	days int,
) ([]domain.UserBalanceSnapshot, error) {
	d, err := load(client, creds, days, scope.Balances)
	if err != nil {
		return nil, err
	}
	return d.BalanceSnapshots(), nil
}

func GetCurrentBalance(
	client *resty.Client,
	creds krakenclient.Credentials,
) (*float64, error) {
	client = authClient(client, creds)

	accounts, err := executors.FetchAccounts(client)
	if err == nil {
		if val, ok := helpers.CurrentBalanceFromAccounts(accounts); ok {
			return &val, nil
		}
	}

	logs, logErr := executors.FetchAllAccountLogSince(client, time.Time{})
	if logErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, logErr
	}
	snapshots := builders.BuildBalanceSnapshots(logs)
	if len(snapshots) == 0 {
		return nil, nil
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})
	val := snapshots[len(snapshots)-1].Balance
	return &val, nil
}

func GetTransactions(
	client *resty.Client,
	creds krakenclient.Credentials,
	days int,
) ([]domain.Transaction, error) {
	d, err := load(client, creds, days, scope.Transactions)
	if err != nil {
		return nil, err
	}
	return d.Transactions(), nil
}

func GetFundings(
	client *resty.Client,
	creds krakenclient.Credentials,
	days int,
) ([]domain.UserFunding, error) {
	d, err := load(client, creds, days, scope.Fundings)
	if err != nil {
		return nil, err
	}
	return d.Fundings(), nil
}

func GetCandles(
	client *resty.Client,
	tickType string,
	symbol string,
	interval string,
	startTime time.Time,
	endTime time.Time,
) ([]models.Candle, error) {
	if client == nil {
		client = krakenclient.NewPublicClient()
	}

	if endTime.Before(startTime) {
		return nil, fmt.Errorf("endTime must be >= startTime")
	}
	if tickType == "" {
		tickType = "trade"
	}

	return executors.FetchCandles(
		client,
		tickType,
		symbol,
		interval,
		startTime.UnixMilli(),
		endTime.UnixMilli(),
	)
}
