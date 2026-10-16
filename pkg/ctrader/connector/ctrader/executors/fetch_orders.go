package executors

import (
	"context"
	"sort"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

func FetchOrders(ctx context.Context, c *connector.Client, from, to time.Time) ([]*pb.ProtoOAOrder, error) {
	session, err := c.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	return fetchOrdersRange(ctx, c, session.CtidTraderAccountID, from, to)
}

func fetchOrdersRange(ctx context.Context, c *connector.Client, accountID int64, from, to time.Time) ([]*pb.ProtoOAOrder, error) {
	fromMs := from.UnixMilli()
	toMs := to.UnixMilli()
	req := &pb.ProtoOAOrderListReq{CtidTraderAccountId: &accountID, FromTimestamp: &fromMs, ToTimestamp: &toMs}
	var res pb.ProtoOAOrderListRes
	if err := c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_ORDER_LIST_REQ, req, &res); err != nil {
		return nil, err
	}
	orders := res.GetOrder()
	if !res.GetHasMore() || !to.After(from) {
		sortOrdersByUpdateTime(orders)
		return orders, nil
	}

	midMs := fromMs + (toMs-fromMs)/2
	if midMs <= fromMs || midMs >= toMs {
		sortOrdersByUpdateTime(orders)
		return orders, nil
	}

	left, err := fetchOrdersRange(ctx, c, accountID, from, time.UnixMilli(midMs))
	if err != nil {
		return nil, err
	}
	right, err := fetchOrdersRange(ctx, c, accountID, time.UnixMilli(midMs+1), to)
	if err != nil {
		return nil, err
	}
	return DedupeOrders(append(left, right...)), nil
}

func DedupeOrders(orders []*pb.ProtoOAOrder) []*pb.ProtoOAOrder {
	seen := make(map[int64]struct{}, len(orders))
	out := make([]*pb.ProtoOAOrder, 0, len(orders))
	for _, order := range orders {
		if order == nil {
			continue
		}
		id := order.GetOrderId()
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, order)
	}
	sortOrdersByUpdateTime(out)
	return out
}

func sortOrdersByUpdateTime(orders []*pb.ProtoOAOrder) {
	sort.Slice(orders, func(i, j int) bool {
		iTime := orders[i].GetUtcLastUpdateTimestamp()
		jTime := orders[j].GetUtcLastUpdateTimestamp()
		if iTime == jTime {
			return orders[i].GetOrderId() < orders[j].GetOrderId()
		}
		return iTime < jTime
	})
}

func FetchOrdersByPositionID(
	ctx context.Context,
	c *connector.Client,
	positionID int64,
	from, to time.Time,
) ([]*pb.ProtoOAOrder, error) {
	session, err := c.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	return fetchOrdersByPositionRange(ctx, c, session.CtidTraderAccountID, positionID, from, to)
}

func fetchOrdersByPositionRange(
	ctx context.Context,
	c *connector.Client,
	accountID, positionID int64,
	from, to time.Time,
) ([]*pb.ProtoOAOrder, error) {
	fromMs := from.UnixMilli()
	if fromMs < 0 {
		fromMs = 0
	}
	toMs := to.UnixMilli()

	req := &pb.ProtoOAOrderListByPositionIdReq{
		CtidTraderAccountId: &accountID,
		PositionId:          &positionID,
		FromTimestamp:       &fromMs,
		ToTimestamp:         &toMs,
	}
	var res pb.ProtoOAOrderListByPositionIdRes
	if err := c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_ORDER_LIST_BY_POSITION_ID_REQ, req, &res); err != nil {
		return nil, err
	}

	orders := res.GetOrder()
	if !res.GetHasMore() || toMs <= fromMs {
		sortOrdersByUpdateTime(orders)
		return orders, nil
	}

	midMs := fromMs + (toMs-fromMs)/2
	if midMs <= fromMs || midMs >= toMs {
		sortOrdersByUpdateTime(orders)
		return orders, nil
	}

	left, err := fetchOrdersByPositionRange(ctx, c, accountID, positionID, time.UnixMilli(fromMs), time.UnixMilli(midMs))
	if err != nil {
		return nil, err
	}
	right, err := fetchOrdersByPositionRange(ctx, c, accountID, positionID, time.UnixMilli(midMs+1), to)
	if err != nil {
		return nil, err
	}
	return DedupeOrders(append(left, right...)), nil
}
