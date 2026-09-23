package okx

import (
	"fmt"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/symbols"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	okxclient "github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

func GetAuthStatus(apiKey, secret, passphrase string) (string, okxclient.Region) {
	region, err := okxclient.CheckAccount(apiKey, secret, passphrase)
	if err != nil {
		return "error", ""
	}

	return "ok", region
}

func load(client *resty.Client, creds okxclient.Credentials, baseURL string, days int, s scope.Scope) (*reconstructor.Dataset, error) {
	okxclient.AttachAuth(client, creds)
	return reconstructor.Load(client, baseURL, helpers.CutoffFromDays(days), s)
}

func Sync(
	client *resty.Client,
	creds okxclient.Credentials,
	baseURL string,
	days int,
) (*domain.Sync, error) {
	d, err := load(client, creds, baseURL, days, scope.All)
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

	key := symbols.Key(pair)
	for i := range positions {
		pos := &positions[i]
		if symbols.Key(pos.Pair) == key && pos.Side == side && pos.CreatedAt.Equal(openedAt) {
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
	d, err := load(client, creds, baseURL, days, scope.Balances)
	if err != nil {
		return nil, err
	}
	return d.BalanceSnapshots(), nil
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
	d, err := load(client, creds, baseURL, days, scope.Transactions)
	if err != nil {
		return nil, err
	}
	return d.Transactions(), nil
}

func GetFundings(
	client *resty.Client,
	creds okxclient.Credentials,
	baseURL string,
	days int,
) ([]domain.UserFunding, error) {
	d, err := load(client, creds, baseURL, days, scope.Fundings)
	if err != nil {
		return nil, err
	}
	return d.Fundings(), nil
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

	instId, err := symbols.Denormalize(client, baseURL, instId)
	if err != nil {
		return nil, err
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
	d, err := load(client, creds, baseURL, days, scope.Closed)
	if err != nil {
		return nil, err
	}
	return d.ClosedPositions()
}

func GetOpenPositions(
	client *resty.Client,
	creds okxclient.Credentials,
	baseURL string,
) ([]domain.OpenPosition, error) {
	d, err := load(client, creds, baseURL, 0, scope.Open)
	if err != nil {
		return nil, err
	}
	return d.OpenPositions()
}

func NormalizeSymbol(symbol string) string {
	return symbols.Normalize(symbol)
}

func DenormalizeSymbol(client *resty.Client, baseURL string, pair string) (string, error) {
	if client == nil {
		client = okxclient.NewBaseClient()
	}
	return symbols.Denormalize(client, baseURL, pair)
}
