package mexc

import (
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	mexcclient "github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

func GetAuthStatus(client *resty.Client, creds mexcclient.Credentials) string {
	mexcclient.AttachAuth(client, creds)

	_, err := executors.FetchUSDTAsset(client)
	if err != nil {
		return "error"
	}

	return "ok"
}

func load(client *resty.Client, creds mexcclient.Credentials, days int, s scope.Scope) (*reconstructor.Dataset, error) {
	mexcclient.AttachAuth(client, creds)
	return reconstructor.Load(client, helpers.CutoffFromDays(days), s)
}

// Sync fetches the account's raw data once and builds every model from it:
// closed and open positions, balance snapshots, the current balance,
// transactions and fundings for the last days (the whole history when
// days <= 0).
func Sync(
	client *resty.Client,
	creds mexcclient.Credentials,
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
		func() (err error) { out.BalanceSnapshots, err = d.BalanceSnapshots(); return err },
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
	creds mexcclient.Credentials,
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
	creds mexcclient.Credentials,
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
	creds mexcclient.Credentials,
) ([]domain.OpenPosition, error) {
	d, err := load(client, creds, 0, scope.Open)
	if err != nil {
		return nil, err
	}
	return d.OpenPositions()
}

func GetBalanceSnapshots(
	client *resty.Client,
	creds mexcclient.Credentials,
	days int,
) ([]domain.UserBalanceSnapshot, error) {
	d, err := load(client, creds, days, scope.Balances)
	if err != nil {
		return nil, err
	}
	return d.BalanceSnapshots()
}

func GetCurrentBalance(
	client *resty.Client,
	creds mexcclient.Credentials,
) (*float64, error) {
	mexcclient.AttachAuth(client, creds)

	val, err := reconstructor.FetchStableEquity(client)
	if err != nil {
		return nil, err
	}

	return &val, nil
}

func GetTransactions(
	client *resty.Client,
	creds mexcclient.Credentials,
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
	creds mexcclient.Credentials,
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
	symbol string,
	interval string,
	startTime time.Time,
	endTime time.Time,
) ([]models.Candle, error) {
	if client == nil {
		client = mexcclient.NewPublicClient()
	}

	if endTime.Before(startTime) {
		return nil, fmt.Errorf("endTime must be >= startTime")
	}

	return executors.FetchCandles(
		client, symbol, interval,
		startTime.UnixMilli(), endTime.UnixMilli(),
	)
}
