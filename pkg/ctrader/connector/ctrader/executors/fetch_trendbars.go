package executors

import (
	"context"
	"sort"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

func FetchTrendbars(
	ctx context.Context,
	c *connector.Client,
	symbolID int64,
	period pb.ProtoOATrendbarPeriod,
	from time.Time,
	to time.Time,
	count uint32,
) ([]*pb.ProtoOATrendbar, error) {
	session, err := c.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	return fetchTrendbarsRange(ctx, c, session.CtidTraderAccountID, symbolID, period, from, to, count)
}

func fetchTrendbarsRange(
	ctx context.Context,
	c *connector.Client,
	accountID int64,
	symbolID int64,
	period pb.ProtoOATrendbarPeriod,
	from time.Time,
	to time.Time,
	count uint32,
) ([]*pb.ProtoOATrendbar, error) {
	fromMs := from.UnixMilli()
	toMs := to.UnixMilli()
	req := &pb.ProtoOAGetTrendbarsReq{
		CtidTraderAccountId: &accountID,
		FromTimestamp:       &fromMs,
		ToTimestamp:         &toMs,
		Period:              &period,
		SymbolId:            &symbolID,
	}
	if count > 0 {
		req.Count = &count
	}
	var res pb.ProtoOAGetTrendbarsRes
	if err := c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_GET_TRENDBARS_REQ, req, &res); err != nil {
		return nil, err
	}
	bars := res.GetTrendbar()
	if count > 0 || !res.GetHasMore() || !to.After(from) {
		sortTrendbarsByTime(bars)
		return bars, nil
	}

	midMs := fromMs + (toMs-fromMs)/2
	if midMs <= fromMs || midMs >= toMs {
		sortTrendbarsByTime(bars)
		return bars, nil
	}

	left, err := fetchTrendbarsRange(ctx, c, accountID, symbolID, period, from, time.UnixMilli(midMs), 0)
	if err != nil {
		return nil, err
	}
	right, err := fetchTrendbarsRange(ctx, c, accountID, symbolID, period, time.UnixMilli(midMs+1), to, 0)
	if err != nil {
		return nil, err
	}
	return dedupeTrendbars(append(left, right...)), nil
}

func dedupeTrendbars(bars []*pb.ProtoOATrendbar) []*pb.ProtoOATrendbar {
	seen := make(map[uint32]struct{}, len(bars))
	out := make([]*pb.ProtoOATrendbar, 0, len(bars))
	for _, bar := range bars {
		if bar == nil {
			continue
		}
		ts := bar.GetUtcTimestampInMinutes()
		if _, ok := seen[ts]; ok {
			continue
		}
		seen[ts] = struct{}{}
		out = append(out, bar)
	}
	sortTrendbarsByTime(out)
	return out
}

func sortTrendbarsByTime(bars []*pb.ProtoOATrendbar) {
	sort.Slice(bars, func(i, j int) bool {
		return bars[i].GetUtcTimestampInMinutes() < bars[j].GetUtcTimestampInMinutes()
	})
}
