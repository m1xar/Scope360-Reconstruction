package ctrader

import (
	"context"
	"fmt"
	registry "github.com/m1xar/scope360-reconstruction/pkg/reconstruction/symbols"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/models"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/parallel"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/window"
)

func newClient(client *connector.Client, cfg connector.Config) *connector.Client {
	if client != nil {
		return client
	}
	return connector.NewClient(cfg)
}

func GetAuthStatus(client *connector.Client, cfg connector.Config) string {
	c := newClient(client, cfg)
	if _, err := c.AuthSession(context.Background()); err != nil {
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
	d, err := load(client, cfg, days, scope.Closed|scope.Open|scope.Balances|scope.Transactions)
	if err != nil {
		return nil, err
	}

	out := &domain.SyncFX{}
	err = parallel.Run(
		func() error { out.Positions = d.ClosedPositions(); return nil },
		func() error { out.OpenPositions = d.OpenPositions(); return nil },
		func() error { out.BalanceSnapshots = d.BalanceSnapshots(); return nil },
		func() error {
			info, err := d.AccountInfo()
			if err != nil {
				return err
			}
			if info != nil {
				out.AccountInfo = *info
			}
			return nil
		},
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
	key := registry.Key(pair)
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
	ctx := context.Background()
	c := newClient(client, cfg)
	trader, err := executors.FetchTrader(ctx, c)
	if err != nil {
		return nil, err
	}
	if trader == nil {
		return nil, nil
	}

	assets, err := executors.FetchAssets(ctx, c)
	if err != nil {
		return nil, err
	}
	currency, ok := helpers.AssetNameByID(assets, trader.GetDepositAssetId())
	if !ok {
		return nil, fmt.Errorf("ctrader deposit asset %d not found", trader.GetDepositAssetId())
	}

	return &domain.FXAccountInfo{
		Balance:  helpers.Money(trader.GetBalance(), trader.GetMoneyDigits()),
		Leverage: uint64(trader.GetLeverageInCents() / 100),
		Currency: currency,
	}, nil
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
	pair string,
	interval string,
	startTime time.Time,
	endTime time.Time,
) ([]models.Candle, error) {
	if endTime.Before(startTime) {
		return nil, fmt.Errorf("endTime must be >= startTime")
	}

	ctx := context.Background()
	c := newClient(client, cfg)
	if _, err := c.EnsureSession(ctx); err != nil {
		return nil, err
	}
	period, err := helpers.TrendbarPeriod(interval)
	if err != nil {
		return nil, err
	}
	symbols, err := executors.FetchLightSymbols(ctx, c)
	if err != nil {
		return nil, err
	}
	symbolID, ok := helpers.SymbolIDByPair(symbols, pair)
	if !ok {
		return nil, fmt.Errorf("ctrader symbol %q not found", pair)
	}
	bars, err := executors.FetchTrendbars(ctx, c, symbolID, period, startTime, endTime, 0)
	if err != nil {
		return nil, err
	}
	return helpers.CandlesFromTrendbars(pair, interval, bars), nil
}

func NormalizeSymbol(symbolName string) string {
	return helpers.NormalizePair(symbolName)
}

func DenormalizeSymbol(client *connector.Client, cfg connector.Config, pair string) (string, error) {
	ctx := context.Background()
	c := newClient(client, cfg)
	symbols, err := executors.FetchLightSymbols(ctx, c)
	if err != nil {
		return "", err
	}
	name, ok := helpers.SymbolNameByPair(symbols, pair)
	if !ok {
		return "", fmt.Errorf("symbol %q not found", pair)
	}
	return name, nil
}
