package executors

import (
	"context"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

const cashFlowWindow = 7 * 24 * time.Hour

func FetchCashFlowHistory(ctx context.Context, c *connector.Client, from, to time.Time) ([]*pb.ProtoOADepositWithdraw, error) {
	session, err := c.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	if to.Before(from) {
		return []*pb.ProtoOADepositWithdraw{}, nil
	}

	accountID := session.CtidTraderAccountID
	fromMs := from.UnixMilli()
	toMs := to.UnixMilli()
	windowMs := cashFlowWindow.Milliseconds()
	out := make([]*pb.ProtoOADepositWithdraw, 0)

	for cursor := fromMs; cursor <= toMs; {
		endMs := cursor + windowMs
		if endMs > toMs {
			endMs = toMs
		}

		req := &pb.ProtoOACashFlowHistoryListReq{
			CtidTraderAccountId: &accountID,
			FromTimestamp:       &cursor,
			ToTimestamp:         &endMs,
		}
		var res pb.ProtoOACashFlowHistoryListRes
		if err := c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_CASH_FLOW_HISTORY_LIST_REQ, req, &res); err != nil {
			return nil, err
		}
		out = append(out, res.GetDepositWithdraw()...)

		if endMs == toMs {
			break
		}
		cursor = endMs + 1
	}

	return out, nil
}
