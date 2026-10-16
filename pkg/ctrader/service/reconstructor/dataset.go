package reconstructor

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/executors"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
)

const candleWorkers = 2

type Dataset struct {
	ctx    context.Context
	client *connector.Client
	days   int
	cutoff *time.Time
	scope  scope.Scope
	now    time.Time

	session *connector.Session
	symbols helpers.Symbols

	deals  []*pb.ProtoOADeal
	orders []*pb.ProtoOAOrder

	reconcile  *pb.ProtoOAReconcileRes
	unrealized *pb.ProtoOAGetPositionUnrealizedPnLRes
	prices     map[int64]helpers.Quote

	trader *pb.ProtoOATrader
	assets []*pb.ProtoOAAsset

	cashFlow []*pb.ProtoOADepositWithdraw

	once   sync.Once
	closed []domain.FXPosition
}

func Load(ctx context.Context, client *connector.Client, days int, s scope.Scope) (*Dataset, error) {
	d := &Dataset{ctx: ctx, client: client, days: days, cutoff: helpers.CutoffFromDays(days), scope: s, now: time.Now().UTC()}

	session, err := client.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	d.session = session

	history := s.Any(scope.Closed | scope.Open | scope.Balances)
	account := s == 0 || s.Any(scope.Open|scope.Balances)

	if history {
		if d.deals, d.orders, err = helpers.LoadHistory(ctx, client, days); err != nil {
			return nil, err
		}
	}
	if history || account {
		if d.reconcile, err = executors.FetchReconcile(ctx, client); err != nil {
			return nil, err
		}
	}
	if account {
		if d.unrealized, err = executors.FetchUnrealizedPnL(ctx, client); err != nil {
			return nil, err
		}
		if d.trader, err = executors.FetchTrader(ctx, client); err != nil {
			return nil, err
		}
		if d.assets, err = executors.FetchAssets(ctx, client); err != nil {
			return nil, err
		}
	}
	if s.Has(scope.Open) {
		d.prices = helpers.FetchCurrentPrices(ctx, client, d.reconcile)
	}
	if s.Has(scope.Transactions) {
		from, to := helpers.HistoryRange(days)
		if d.cashFlow, err = executors.FetchCashFlowHistory(ctx, client, from, to); err != nil {
			return nil, err
		}
	}
	if d.symbols, err = d.loadSymbols(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Dataset) loadSymbols() (helpers.Symbols, error) {
	names, err := executors.FetchSymbolNames(d.ctx, d.client)
	if err != nil {
		return helpers.Symbols{}, err
	}
	used := make(map[int64]bool)
	for _, deal := range d.deals {
		used[deal.GetSymbolId()] = true
	}
	for _, order := range d.orders {
		used[order.GetTradeData().GetSymbolId()] = true
	}
	for _, pos := range d.reconcile.GetPosition() {
		used[pos.GetTradeData().GetSymbolId()] = true
	}
	for _, order := range d.reconcile.GetOrder() {
		used[order.GetTradeData().GetSymbolId()] = true
	}
	ids := make([]int64, 0, len(used))
	for id := range used {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	full, err := executors.FetchSymbols(d.ctx, d.client, ids)
	if err != nil {
		return helpers.Symbols{}, err
	}
	return helpers.NewSymbols(names, full), nil
}

func (d *Dataset) openIDs() map[int64]bool {
	out := make(map[int64]bool)
	for _, pos := range d.reconcile.GetPosition() {
		out[pos.GetPositionId()] = true
	}
	return out
}

func (d *Dataset) build() {
	d.once.Do(func() {
		positions := builders.BuildFXPositions(d.deals, d.orders, d.openIDs(), d.symbols, d.session)
		if d.scope.Has(scope.Closed) {
			fx.EnrichMAEMFE(positions, helpers.BarFetcher(d.ctx, d.client, d.symbols), candleWorkers)
		}
		d.closed = fx.ClosedAfter(positions, d.cutoff)
	})
}

func (d *Dataset) ClosedPositions() []domain.FXPosition {
	d.build()
	return d.closed
}

func (d *Dataset) OpenPositions() []domain.FXOpenPosition {
	open := builders.BuildOpenPositions(d.reconcile, d.deals, d.unrealized, d.prices, d.symbols, d.session)
	fx.EnrichOpenMAEMFE(open, d.now, helpers.BarFetcher(d.ctx, d.client, d.symbols), candleWorkers)
	return open
}

func (d *Dataset) PendingOrders() []domain.FXOrder {
	return builders.BuildPendingOrders(d.reconcile, d.symbols, d.session)
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
	info := &domain.FXAccountInfo{
		Balance:  helpers.Money(d.trader.GetBalance(), d.trader.GetMoneyDigits()),
		Leverage: uint64(d.trader.GetLeverageInCents() / 100),
		Currency: currency,
		IsDemo:   !d.session.IsLive,
	}
	var unrealized, margin float64
	if d.unrealized != nil {
		for _, item := range d.unrealized.GetPositionUnrealizedPnL() {
			unrealized += helpers.Money(item.GetNetUnrealizedPnL(), d.unrealized.GetMoneyDigits())
		}
	}
	for _, pos := range d.reconcile.GetPosition() {
		digits := d.session.MoneyDigits
		if pos.MoneyDigits != nil {
			digits = pos.GetMoneyDigits()
		}
		margin += helpers.Money(int64(pos.GetUsedMargin()), digits)
	}
	info.Equity = fx.Round8(info.Balance + unrealized)
	info.Margin = fx.Round8(margin)
	info.FreeMargin = fx.Round8(info.Equity - margin)
	return info, nil
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
