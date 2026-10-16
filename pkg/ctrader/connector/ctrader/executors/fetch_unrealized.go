package executors

import (
	"context"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

func FetchUnrealizedPnL(ctx context.Context, c *connector.Client) (*pb.ProtoOAGetPositionUnrealizedPnLRes, error) {
	session, err := c.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	accountID := session.CtidTraderAccountID
	req := &pb.ProtoOAGetPositionUnrealizedPnLReq{CtidTraderAccountId: &accountID}
	var res pb.ProtoOAGetPositionUnrealizedPnLRes
	if err := c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_GET_POSITION_UNREALIZED_PNL_REQ, req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}
