package helpers

import (
	"context"
	"sort"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/executors"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

func LoadHistory(ctx context.Context, c *connector.Client, days int) ([]*pb.ProtoOADeal, []*pb.ProtoOAOrder, error) {
	from, to := HistoryRange(days)
	deals, err := executors.FetchDeals(ctx, c, from, to)
	if err != nil {
		return nil, nil, err
	}
	orders, err := executors.FetchOrders(ctx, c, from, to)
	if err != nil {
		return nil, nil, err
	}
	return backfillTruncatedPositions(ctx, c, deals, orders, to)
}

func backfillTruncatedPositions(
	ctx context.Context,
	c *connector.Client,
	deals []*pb.ProtoOADeal,
	orders []*pb.ProtoOAOrder,
	to time.Time,
) ([]*pb.ProtoOADeal, []*pb.ProtoOAOrder, error) {
	truncated := positionsMissingOpeningDeal(deals)
	if len(truncated) == 0 {
		return deals, orders, nil
	}

	from := time.UnixMilli(0)

	for _, positionID := range truncated {
		extraDeals, err := executors.FetchDealsByPositionID(ctx, c, positionID, from, to)
		if err != nil {
			return nil, nil, err
		}
		deals = append(deals, extraDeals...)

		extraOrders, err := executors.FetchOrdersByPositionID(ctx, c, positionID, from, to)
		if err != nil {
			return nil, nil, err
		}
		orders = append(orders, extraOrders...)
	}

	return executors.DedupeDeals(deals), executors.DedupeOrders(orders), nil
}

func positionsMissingOpeningDeal(deals []*pb.ProtoOADeal) []int64 {
	type coverage struct {
		opened bool
		closed bool
	}

	seen := make(map[int64]*coverage)
	order := make([]int64, 0)

	for _, deal := range deals {
		if deal == nil || deal.GetDealStatus() != pb.ProtoOADealStatus_FILLED {
			continue
		}

		positionID := deal.GetPositionId()
		cov := seen[positionID]
		if cov == nil {
			cov = &coverage{}
			seen[positionID] = cov
			order = append(order, positionID)
		}

		if deal.GetClosePositionDetail() != nil {
			cov.closed = true
			continue
		}
		cov.opened = true
	}

	out := make([]int64, 0)
	for _, positionID := range order {
		cov := seen[positionID]
		if cov.closed && !cov.opened {
			out = append(out, positionID)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

type Quote struct {
	Bid float64
	Ask float64
}

func FetchCurrentPrices(ctx context.Context, c *connector.Client, reconcile *pb.ProtoOAReconcileRes) map[int64]Quote {
	prices := make(map[int64]Quote)
	if reconcile == nil {
		return prices
	}
	for _, pos := range reconcile.GetPosition() {
		symbolID := pos.GetTradeData().GetSymbolId()
		if _, done := prices[symbolID]; done {
			continue
		}
		spot, err := c.FetchSpot(ctx, symbolID)
		if err != nil || spot == nil {
			continue
		}
		prices[symbolID] = Quote{Bid: SpotPrice(spot.GetBid()), Ask: SpotPrice(spot.GetAsk())}
	}
	return prices
}

func AssetNameByID(assets []*pb.ProtoOAAsset, id int64) (string, bool) {
	for _, asset := range assets {
		if asset == nil || asset.GetAssetId() != id {
			continue
		}
		return asset.GetName(), true
	}
	return "", false
}
