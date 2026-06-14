package ctrader

import (
	"context"
	"fmt"
	"sort"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/models"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
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

func GetBuiltPositions(
	client *connector.Client,
	cfg connector.Config,
	days int,
) ([]domain.FXPosition, error) {
	ctx := context.Background()
	c := newClient(client, cfg)

	deals, orders, symbols, session, err := loadHistory(ctx, c, days)
	if err != nil {
		return nil, err
	}
	positions := builders.BuildFXPositions(deals, orders, symbols, session)
	enrichFXMAEMFE(ctx, c, positions, symbols)
	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		filtered := positions[:0]
		for _, pos := range positions {
			if pos.ClosedAt != nil && !pos.ClosedAt.Before(*cutoff) {
				filtered = append(filtered, pos)
			}
		}
		positions = filtered
	}
	return positions, nil
}

func GetClosedPositionByExactMatch(
	client *connector.Client,
	cfg connector.Config,
	pair string,
	openedAt time.Time,
	side string,
) (*domain.FXPosition, error) {
	positions, err := GetBuiltPositions(client, cfg, 0)
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
	client *connector.Client,
	cfg connector.Config,
) ([]domain.OpenPosition, error) {
	ctx := context.Background()
	c := newClient(client, cfg)

	session, err := c.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	symbols, err := executors.FetchSymbolNames(ctx, c)
	if err != nil {
		return nil, err
	}
	reconcile, err := executors.FetchReconcile(ctx, c)
	if err != nil {
		return nil, err
	}
	currentPrices := fetchCurrentPrices(ctx, c, reconcile)
	return builders.BuildOpenPositions(reconcile, symbols, currentPrices, session), nil
}

func GetBalanceSnapshots(
	client *connector.Client,
	cfg connector.Config,
	days int,
) ([]domain.UserBalanceSnapshot, error) {
	positions, err := GetBuiltPositions(client, cfg, 0)
	if err != nil {
		return nil, err
	}
	snapshots := builders.BuildBalanceSnapshots(positions)
	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		filtered := snapshots[:0]
		for _, snapshot := range snapshots {
			if !snapshot.CreatedAt.Before(*cutoff) {
				filtered = append(filtered, snapshot)
			}
		}
		snapshots = filtered
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt) })
	return snapshots, nil
}

func GetCurrentBalance(
	client *connector.Client,
	cfg connector.Config,
) (*float64, error) {
	positions, err := GetBuiltPositions(client, cfg, 0)
	if err != nil {
		return nil, err
	}
	if len(positions) > 0 {
		sort.Slice(positions, func(i, j int) bool {
			if positions[i].ClosedAt == nil {
				return true
			}
			if positions[j].ClosedAt == nil {
				return false
			}
			return positions[i].ClosedAt.Before(*positions[j].ClosedAt)
		})
		last := positions[len(positions)-1]
		balance := last.BalanceInit + last.NetPnl
		return &balance, nil
	}

	ctx := context.Background()
	trader, err := executors.FetchTrader(ctx, newClient(client, cfg))
	if err != nil {
		return nil, err
	}
	if trader == nil {
		return nil, nil
	}
	balance := helpers.Money(trader.GetBalance(), trader.GetMoneyDigits())
	return &balance, nil
}

func GetSwaps(
	client *connector.Client,
	cfg connector.Config,
	days int,
) ([]domain.UserSwap, error) {
	ctx := context.Background()
	deals, _, symbols, session, err := loadHistory(ctx, newClient(client, cfg), days)
	if err != nil {
		return nil, err
	}
	swaps := builders.BuildSwaps(deals, symbols, session)
	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		filtered := swaps[:0]
		for _, swap := range swaps {
			if !swap.CreatedAt.Before(*cutoff) {
				filtered = append(filtered, swap)
			}
		}
		swaps = filtered
	}
	return swaps, nil
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

func loadHistory(ctx context.Context, c *connector.Client, days int) ([]*pb.ProtoOADeal, []*pb.ProtoOAOrder, map[int64]string, *connector.Session, error) {
	session, err := c.EnsureSession(ctx)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	from, to := helpers.HistoryRange(days)
	deals, err := executors.FetchDeals(ctx, c, from, to)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	orders, err := executors.FetchOrders(ctx, c, from, to)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	symbols, err := executors.FetchSymbolNames(ctx, c)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return deals, orders, symbols, session, nil
}

func enrichFXMAEMFE(ctx context.Context, c *connector.Client, positions []domain.FXPosition, symbols map[int64]string) {
	if len(positions) == 0 {
		return
	}
	idByPair := make(map[string]int64, len(symbols))
	for id, name := range symbols {
		idByPair[name] = id
	}
	period := pb.ProtoOATrendbarPeriod_M1
	for i := range positions {
		pos := &positions[i]
		if pos.ClosedAt == nil {
			continue
		}
		symbolID, ok := idByPair[pos.Pair]
		if !ok {
			continue
		}
		bars, err := executors.FetchTrendbars(ctx, c, symbolID, period, pos.CreatedAt, *pos.ClosedAt, 0)
		if err != nil {
			continue
		}
		candles := helpers.CandlesFromTrendbars(pos.Pair, "M1", bars)
		high, low := helpers.CandleHighLow(candles)
		helpers.ApplyFXMAEMFE(pos, high, low)
	}
}

func fetchCurrentPrices(ctx context.Context, c *connector.Client, reconcile *pb.ProtoOAReconcileRes) map[int64]float64 {
	if reconcile == nil {
		return map[int64]float64{}
	}
	symbolIDs := make(map[int64]struct{})
	for _, pos := range reconcile.GetPosition() {
		if pos == nil || pos.GetTradeData() == nil {
			continue
		}
		symbolIDs[pos.GetTradeData().GetSymbolId()] = struct{}{}
	}
	prices := make(map[int64]float64, len(symbolIDs))
	for symbolID := range symbolIDs {
		spot, err := executors.FetchSpot(ctx, c, symbolID)
		if err != nil || spot == nil {
			continue
		}
		price := helpers.SpotPrice(spot.GetBid())
		if price == 0 {
			price = helpers.SpotPrice(spot.GetAsk())
		}
		if price != 0 {
			prices[symbolID] = price
		}
	}
	return prices
}
