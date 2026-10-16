package executors

import (
	"context"
	"sort"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

const maxDealRows int32 = 500

func FetchDeals(ctx context.Context, c *connector.Client, from, to time.Time) ([]*pb.ProtoOADeal, error) {
	session, err := c.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	return fetchDealsRange(ctx, c, session.CtidTraderAccountID, from, to)
}

func fetchDealsRange(ctx context.Context, c *connector.Client, accountID int64, from, to time.Time) ([]*pb.ProtoOADeal, error) {
	fromMs := from.UnixMilli()
	toMs := to.UnixMilli()
	maxRows := maxDealRows
	req := &pb.ProtoOADealListReq{CtidTraderAccountId: &accountID, FromTimestamp: &fromMs, ToTimestamp: &toMs, MaxRows: &maxRows}
	var res pb.ProtoOADealListRes
	if err := c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_DEAL_LIST_REQ, req, &res); err != nil {
		return nil, err
	}
	deals := res.GetDeal()
	if !res.GetHasMore() || !to.After(from) {
		sortDealsByExecutionTime(deals)
		return deals, nil
	}

	midMs := fromMs + (toMs-fromMs)/2
	if midMs <= fromMs || midMs >= toMs {
		sortDealsByExecutionTime(deals)
		return deals, nil
	}

	left, err := fetchDealsRange(ctx, c, accountID, from, time.UnixMilli(midMs))
	if err != nil {
		return nil, err
	}
	right, err := fetchDealsRange(ctx, c, accountID, time.UnixMilli(midMs+1), to)
	if err != nil {
		return nil, err
	}
	return DedupeDeals(append(left, right...)), nil
}

func DedupeDeals(deals []*pb.ProtoOADeal) []*pb.ProtoOADeal {
	seen := make(map[int64]struct{}, len(deals))
	out := make([]*pb.ProtoOADeal, 0, len(deals))
	for _, deal := range deals {
		if deal == nil {
			continue
		}
		id := deal.GetDealId()
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, deal)
	}
	sortDealsByExecutionTime(out)
	return out
}

func sortDealsByExecutionTime(deals []*pb.ProtoOADeal) {
	sort.Slice(deals, func(i, j int) bool {
		if deals[i].GetExecutionTimestamp() == deals[j].GetExecutionTimestamp() {
			return deals[i].GetDealId() < deals[j].GetDealId()
		}
		return deals[i].GetExecutionTimestamp() < deals[j].GetExecutionTimestamp()
	})
}

func FetchDealsByPositionID(
	ctx context.Context,
	c *connector.Client,
	positionID int64,
	from, to time.Time,
) ([]*pb.ProtoOADeal, error) {
	session, err := c.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	return fetchDealsByPositionRange(ctx, c, session.CtidTraderAccountID, positionID, from, to)
}

func fetchDealsByPositionRange(
	ctx context.Context,
	c *connector.Client,
	accountID, positionID int64,
	from, to time.Time,
) ([]*pb.ProtoOADeal, error) {
	fromMs := from.UnixMilli()
	if fromMs < 0 {
		fromMs = 0
	}
	toMs := to.UnixMilli()

	req := &pb.ProtoOADealListByPositionIdReq{
		CtidTraderAccountId: &accountID,
		PositionId:          &positionID,
		FromTimestamp:       &fromMs,
		ToTimestamp:         &toMs,
	}
	var res pb.ProtoOADealListByPositionIdRes
	if err := c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_DEAL_LIST_BY_POSITION_ID_REQ, req, &res); err != nil {
		return nil, err
	}

	deals := res.GetDeal()
	if !res.GetHasMore() || toMs <= fromMs {
		sortDealsByExecutionTime(deals)
		return deals, nil
	}

	midMs := fromMs + (toMs-fromMs)/2
	if midMs <= fromMs || midMs >= toMs {
		sortDealsByExecutionTime(deals)
		return deals, nil
	}

	left, err := fetchDealsByPositionRange(ctx, c, accountID, positionID, time.UnixMilli(fromMs), time.UnixMilli(midMs))
	if err != nil {
		return nil, err
	}
	right, err := fetchDealsByPositionRange(ctx, c, accountID, positionID, time.UnixMilli(midMs+1), to)
	if err != nil {
		return nil, err
	}
	return DedupeDeals(append(left, right...)), nil
}
