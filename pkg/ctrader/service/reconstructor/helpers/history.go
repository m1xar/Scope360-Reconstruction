package helpers

import (
	"context"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/executors"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
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
	symbols, err := executors.FetchSymbolNames(ctx, c)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return deals, orders, symbols, session, nil
}

func EnrichFXMAEMFE(ctx context.Context, c *connector.Client, positions []domain.FXPosition, symbols map[int64]string) {
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
		candles := CandlesFromTrendbars(pos.Pair, "M1", bars)
		high, low := CandleHighLow(candles)
		ApplyFXMAEMFE(pos, high, low)
	}
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
