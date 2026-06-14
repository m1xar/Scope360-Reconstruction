package executors

import (
	"context"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

func FetchTrader(ctx context.Context, c *connector.Client) (*pb.ProtoOATrader, error) {
	session, err := c.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	accountID := session.CtidTraderAccountID
	req := &pb.ProtoOATraderReq{CtidTraderAccountId: &accountID}
	var res pb.ProtoOATraderRes
	if err := c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_TRADER_REQ, req, &res); err != nil {
		return nil, err
	}
	return res.GetTrader(), nil
}
