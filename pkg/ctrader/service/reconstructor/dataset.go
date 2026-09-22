package reconstructor

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
)

type Dataset struct {
	ctx    context.Context
	client *connector.Client
	days   int
	cutoff *time.Time
	scope  scope.Scope

	session *connector.Session
	symbols map[int64]string

	deals  []*pb.ProtoOADeal
	orders []*pb.ProtoOAOrder

	reconcile *pb.ProtoOAReconcileRes
	prices    map[int64]float64

	trader *pb.ProtoOATrader
	assets []*pb.ProtoOAAsset

	cashFlow []*pb.ProtoOADepositWithdraw

	closedOnce sync.Once
	closed     []domain.FXPosition
}

func Load(ctx context.Context, client *connector.Client, days int, s scope.Scope) (*Dataset, error) {
	d := &Dataset{ctx: ctx, client: client, days: days, cutoff: helpers.CutoffFromDays(days), scope: s}

	session, err := client.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	d.session = session

	if s.Any(scope.Closed | scope.Balances) {
		deals, orders, symbols, _, err := helpers.LoadHistory(ctx, client, days)
		if err != nil {
			return nil, err
		}
		d.deals, d.orders, d.symbols = deals, orders, symbols
	}
	if s.Has(scope.Open) {
		if d.symbols == nil {
			symbols, err := executors.FetchSymbolNames(ctx, client)
			if err != nil {
				return nil, err
			}
			d.symbols = symbols
		}
		reconcile, err := executors.FetchReconcile(ctx, client)
		if err != nil {
			return nil, err
		}
		d.reconcile = reconcile
		d.prices = helpers.FetchCurrentPrices(ctx, client, reconcile)
	}
	if s.Has(scope.Balances) {
		trader, err := executors.FetchTrader(ctx, client)
		if err != nil {
			return nil, err
		}
		assets, err := executors.FetchAssets(ctx, client)
		if err != nil {
			return nil, err
		}
		d.trader, d.assets = trader, assets
	}
	if s.Has(scope.Transactions) {
		from, to := helpers.HistoryRange(days)
		cashFlow, err := executors.FetchCashFlowHistory(ctx, client, from, to)
		if err != nil {
			return nil, err
		}
		d.cashFlow = cashFlow
	}
	return d, nil
}

func (d *Dataset) ClosedPositions() []domain.FXPosition {
	d.closedOnce.Do(func() {
		positions := builders.BuildFXPositions(d.deals, d.orders, d.symbols, d.session)
		if d.scope.Has(scope.Closed) {
			helpers.EnrichFXMAEMFE(d.ctx, d.client, positions, d.symbols)
		}
		if d.cutoff != nil {
			kept := positions[:0]
			for _, pos := range positions {
				if pos.ClosedAt != nil && !pos.ClosedAt.Before(*d.cutoff) {
					kept = append(kept, pos)
				}
			}
			positions = kept
		}
		d.closed = positions
	})
	return d.closed
}

func (d *Dataset) OpenPositions() []domain.FXOpenPosition {
	return builders.BuildOpenPositions(d.reconcile, d.symbols, d.prices, d.session)
}

func (d *Dataset) BalanceSnapshots() []domain.UserBalanceSnapshot {
	snapshots := builders.BuildBalanceSnapshots(d.ClosedPositions())
	if d.cutoff != nil {
		filtered := snapshots[:0]
		for _, snapshot := range snapshots {
			if !snapshot.CreatedAt.Before(*d.cutoff) {
				filtered = append(filtered, snapshot)
			}
		}
		snapshots = filtered
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt) })
	return snapshots
}

func (d *Dataset) AccountInfo() (*domain.FXAccountInfo, error) {
	if d.trader == nil {
		return nil, nil
	}
	currency, ok := helpers.AssetNameByID(d.assets, d.trader.GetDepositAssetId())
	if !ok {
		return nil, fmt.Errorf("ctrader deposit asset %d not found", d.trader.GetDepositAssetId())
	}
	return &domain.FXAccountInfo{
		Balance:  helpers.Money(d.trader.GetBalance(), d.trader.GetMoneyDigits()),
		Leverage: uint64(d.trader.GetLeverageInCents() / 100),
		Currency: currency,
	}, nil
}

func (d *Dataset) Transactions() []domain.Transaction {
	transactions := builders.BuildTransactions(d.cashFlow)
	if d.cutoff == nil {
		return transactions
	}
	filtered := transactions[:0]
	for _, tx := range transactions {
		if !tx.Time.Before(*d.cutoff) {
			filtered = append(filtered, tx)
		}
	}
	return filtered
}
