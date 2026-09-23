package blofin

import (
	"fmt"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/symbols"
	"time"

	"github.com/go-resty/resty/v2"
	blofinclient "github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
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

func load(client *resty.Client, creds blofinclient.Credentials, days int, s scope.Scope) (*reconstructor.Dataset, error) {
	blofinclient.AttachAuth(client, creds)
	return reconstructor.Load(client, blofinclient.BaseURL, helpers.CutoffFromDays(days), s)
}

func Sync(
	client *resty.Client,
	creds blofinclient.Credentials,
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
	creds blofinclient.Credentials,
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
	creds blofinclient.Credentials,
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
	creds blofinclient.Credentials,
) ([]domain.OpenPosition, error) {
	d, err := load(client, creds, 0, scope.Open)
	if err != nil {
		return nil, err
	}
	return d.OpenPositions()
}

func GetBalanceSnapshots(
	client *resty.Client,
	creds blofinclient.Credentials,
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
	d, err := load(client, creds, days, scope.Transactions)
	if err != nil {
		return nil, err
	}
	return d.Transactions(), nil
}

func GetFundings(
	client *resty.Client,
	creds blofinclient.Credentials,
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

	instID, err := symbols.Denormalize(client, instID)
	if err != nil {
		return nil, err
	}

	return executors.FetchCandles(
		client, blofinclient.BaseURL, instID, bar,
		startTime.UnixMilli(), endTime.UnixMilli(),
	)
}

func NormalizeSymbol(symbol string) string {
	return symbols.Normalize(symbol)
}

func DenormalizeSymbol(client *resty.Client, pair string) (string, error) {
	if client == nil {
		client = blofinclient.NewBaseClient()
	}
	return symbols.Denormalize(client, pair)
}
