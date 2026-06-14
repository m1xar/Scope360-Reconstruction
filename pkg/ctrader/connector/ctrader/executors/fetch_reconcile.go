package executors

import (
	"context"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

func FetchReconcile(ctx context.Context, c *connector.Client) (*pb.ProtoOAReconcileRes, error) {
	session, err := c.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	accountID := session.CtidTraderAccountID
	returnProtectionOrders := true
	req := &pb.ProtoOAReconcileReq{CtidTraderAccountId: &accountID, ReturnProtectionOrders: &returnProtectionOrders}
	var res pb.ProtoOAReconcileRes
	if err := c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_RECONCILE_REQ, req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}
