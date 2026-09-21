package helpers

import (
	"context"
	"sort"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/models"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/candlespan"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/excursion"
)

func LoadHistory(ctx context.Context, c *connector.Client, days int) ([]*pb.ProtoOADeal, []*pb.ProtoOAOrder, map[int64]string, *connector.Session, error) {
	session, err := c.EnsureSession(ctx)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	from, to := HistoryRange(days)
	deals, err := executors.FetchDeals(ctx, c, from, to)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	orders, err := executors.FetchOrders(ctx, c, from, to)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	deals, orders, err = backfillTruncatedPositions(ctx, c, deals, orders, to)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	symbols, err := executors.FetchSymbolNames(ctx, c)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return deals, orders, symbols, session, nil
}

func backfillTruncatedPositions(
	ctx context.Context,
	c *connector.Client,
	deals []*pb.ProtoOADeal,
	orders []*pb.ProtoOAOrder,
	to time.Time,
) ([]*pb.ProtoOADeal, []*pb.ProtoOAOrder, error) {
	truncated := PositionsMissingOpeningDeal(deals)
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

func PositionsMissingOpeningDeal(deals []*pb.ProtoOADeal) []int64 {
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

func EnrichFXMAEMFE(ctx context.Context, c *connector.Client, positions []domain.FXPosition, symbols map[int64]string) {
	if len(positions) == 0 {
		return
	}
	idByPair := make(map[string]int64, len(symbols))
	for id, name := range symbols {
		idByPair[name] = id
	}
	for i := range positions {
		pos := &positions[i]
		if pos.ClosedAt == nil {
			continue
		}
		symbolID, ok := idByPair[pos.Pair]
		if !ok {
			continue
		}
		candles, err := trendbarsForSpan(ctx, c, symbolID, pos.Pair, pos.CreatedAt, *pos.ClosedAt)
		if err != nil {
			continue
		}
		inside := candles[:0]
		for _, candle := range candles {
			if excursion.Inside(candle.OpenTime.UnixMilli(), excursion.IntervalMs(candle.Interval), pos.CreatedAt.UnixMilli(), pos.ClosedAt.UnixMilli()) {
				inside = append(inside, candle)
			}
		}
		high, low := CandleHighLow(inside)
		ApplyFXMAEMFE(pos, high, low)
	}
}

func trendbarsForSpan(
	ctx context.Context,
	c *connector.Client,
	symbolID int64,
	pair string,
	from, to time.Time,
) ([]models.Candle, error) {
	var out []models.Candle

	for _, segment := range candlespan.Split(from.UnixMilli(), to.UnixMilli()) {
		period, err := TrendbarPeriod(segment.Interval)
		if err != nil {
			return nil, err
		}

		bars, err := executors.FetchTrendbars(
			ctx, c, symbolID, period,
			time.UnixMilli(segment.StartMs), time.UnixMilli(segment.EndMs), 0,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, CandlesFromTrendbars(pair, segment.Interval, bars)...)
	}

	return out, nil
}

func FetchCurrentPrices(ctx context.Context, c *connector.Client, reconcile *pb.ProtoOAReconcileRes) map[int64]float64 {
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
		price := SpotPrice(spot.GetBid())
		if price == 0 {
			price = SpotPrice(spot.GetAsk())
		}
		if price != 0 {
			prices[symbolID] = price
		}
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
