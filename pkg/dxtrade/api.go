package dxtrade

import (
	"context"
	"fmt"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	connector "github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
	registry "github.com/m1xar/scope360-reconstruction/pkg/reconstruction/symbols"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

func newClient(client *connector.Client, cfg connector.Config) *connector.Client {
	if client != nil {
		return client
	}
	return connector.NewClient(cfg)
}

func GetAuthStatus(client *connector.Client, cfg connector.Config) string {
	if _, err := newClient(client, cfg).Account(context.Background()); err != nil {
		return "error"
	}
	return "ok"
}

func load(client *connector.Client, cfg connector.Config, days int, s scope.Scope) (*reconstructor.Dataset, error) {
	return reconstructor.Load(context.Background(), newClient(client, cfg), days, s)
}

func Sync(
	client *connector.Client,
	cfg connector.Config,
	days int,
) (*domain.SyncFX, error) {
	c := newClient(client, cfg)
	if client == nil {
		defer c.Logout(context.Background())
	}
	d, err := load(c, cfg, days, scope.Closed|scope.Open|scope.Balances|scope.Transactions)
	if err != nil {
		return nil, err
	}

	out := &domain.SyncFX{}
	err = parallel.Run(
		func() error { out.Positions = d.ClosedPositions(); return nil },
		func() error { out.OpenPositions = d.OpenPositions(); return nil },
		func() error { out.PendingOrders = d.PendingOrders(); return nil },
		func() error { out.BalanceSnapshots = d.BalanceSnapshots(); return nil },
		func() error { out.AccountInfo = d.AccountInfo(); return nil },
		func() error { out.Transactions = d.Transactions(); return nil },
	)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func GetBuiltPositions(
	client *connector.Client,
	cfg connector.Config,
	days int,
) ([]domain.FXPosition, error) {
	d, err := load(client, cfg, days, scope.Closed)
	if err != nil {
		return nil, err
	}
	return d.ClosedPositions(), nil
}

func GetClosedPositionByExactMatch(
	client *connector.Client,
	cfg connector.Config,
	pair string,
	openedAt time.Time,
	side string,
) (*domain.FXPosition, error) {
	positions, err := GetBuiltPositions(client, cfg, window.DaysSince(openedAt))
	if err != nil {
		return nil, err
	}
	key := registry.Key(fx.NormalizePair(pair))
	for i := range positions {
		pos := &positions[i]
		if registry.Key(pos.Pair) == key && pos.Side == side && pos.CreatedAt.Equal(openedAt) {
			return pos, nil
		}
	}
	return nil, nil
}

func GetOpenPositions(
	client *connector.Client,
	cfg connector.Config,
) ([]domain.FXOpenPosition, error) {
	d, err := load(client, cfg, 0, scope.Open)
	if err != nil {
		return nil, err
	}
	return d.OpenPositions(), nil
}

func GetPendingOrders(
	client *connector.Client,
	cfg connector.Config,
) ([]domain.FXOrder, error) {
	d, err := load(client, cfg, 0, scope.Open)
	if err != nil {
		return nil, err
	}
	return d.PendingOrders(), nil
}

func GetBalanceSnapshots(
	client *connector.Client,
	cfg connector.Config,
	days int,
) ([]domain.UserBalanceSnapshot, error) {
	d, err := load(client, cfg, days, scope.Balances)
	if err != nil {
		return nil, err
	}
	return d.BalanceSnapshots(), nil
}

func GetAccountInfo(
	client *connector.Client,
	cfg connector.Config,
) (*domain.FXAccountInfo, error) {
	d, err := load(client, cfg, 0, 0)
	if err != nil {
		return nil, err
	}
	info := d.AccountInfo()
	return &info, nil
}

func GetTransactions(
	client *connector.Client,
	cfg connector.Config,
	days int,
) ([]domain.Transaction, error) {
	d, err := load(client, cfg, days, scope.Transactions)
	if err != nil {
		return nil, err
	}
	return d.Transactions(), nil
}

func GetCandles(
	client *connector.Client,
	cfg connector.Config,
	symbol string,
	interval string,
	startTime time.Time,
	endTime time.Time,
) ([]models.Candle, error) {
	if endTime.Before(startTime) {
		return nil, fmt.Errorf("endTime must be >= startTime")
	}
	c := newClient(client, cfg)
	name, err := DenormalizeSymbol(c, cfg, symbol)
	if err != nil {
		return nil, err
	}
	events, err := executors.FetchCandles(context.Background(), c, name, interval, startTime, endTime)
	if err != nil {
		return nil, err
	}
	return helpers.Candles(fx.NormalizePair(name), interval, events), nil
}

func GetContractSizes(
	client *connector.Client,
	cfg connector.Config,
) ([]domain.FXSymbolSpec, error) {
	d, err := load(client, cfg, 0, 0)
	if err != nil {
		return nil, err
	}
	return d.ContractSizes()
}

func NormalizeSymbol(symbolName string) string {
	return fx.NormalizePair(symbolName)
}

func DenormalizeSymbol(client *connector.Client, cfg connector.Config, pair string) (string, error) {
	instruments, err := executors.FetchInstruments(context.Background(), newClient(client, cfg))
	if err != nil {
		return "", err
	}
	name, ok := helpers.SymbolByPair(instruments, pair)
	if !ok {
		return "", fmt.Errorf("symbol %q not found", pair)
	}
	return name, nil
}
