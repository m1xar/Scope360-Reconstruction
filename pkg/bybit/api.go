package bybit

import (
	"fmt"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/symbols"
	"time"

	"github.com/go-resty/resty/v2"
	bybitclient "github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit/models"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
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

func load(client *resty.Client, creds bybitclient.Credentials, days int, s scope.Scope) (*reconstructor.Dataset, error) {
	return reconstructor.Load(authClient(client, creds), helpers.CutoffFromDays(days), s)
}

func Sync(
	client *resty.Client,
	creds bybitclient.Credentials,
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
	creds bybitclient.Credentials,
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
	creds bybitclient.Credentials,
	pair string,
	openedAt time.Time,
	side string,
) (*domain.Position, error) {
	positions, err := GetBuiltPositions(client, creds, window.DaysSince(openedAt))
	if err != nil {
		return nil, err
	}

	key := symbols.Key(pair)
	for i := range positions {
		pos := &positions[i]
		if symbols.Key(pos.Pair) == key && pos.Side == side && pos.CreatedAt.Equal(openedAt) {
			return pos, nil
		}
	}
	return nil, nil
}

func GetOpenPositions(
	client *resty.Client,
	creds bybitclient.Credentials,
) ([]domain.OpenPosition, error) {
	d, err := load(client, creds, 0, scope.Open)
	if err != nil {
		return nil, err
	}
	return d.OpenPositions()
}

func GetBalanceSnapshots(
	client *resty.Client,
	creds bybitclient.Credentials,
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
	d, err := load(client, creds, days, scope.Transactions)
	if err != nil {
		return nil, err
	}
	return d.Transactions(), nil
}

func GetFundings(
	client *resty.Client,
	creds bybitclient.Credentials,
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
		client = bybitclient.NewBaseClient()
	}

	if endTime.Before(startTime) {
		return nil, fmt.Errorf("endTime must be >= startTime")
	}

	if resolved, err := symbols.Denormalize(client, symbol); err == nil {
		symbol = resolved
	}

	return executors.FetchCandles(
		client, symbol, interval,
		startTime.UnixMilli(), endTime.UnixMilli(),
	)
}

func NormalizeSymbol(symbol string) string {
	return symbols.Normalize(symbol)
}

func DenormalizeSymbol(client *resty.Client, pair string) (string, error) {
	if client == nil {
		client = bybitclient.NewBaseClient()
	}
	return symbols.Denormalize(client, pair)
}
