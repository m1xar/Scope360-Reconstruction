package executors

import (
	"context"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

func FetchAssets(ctx context.Context, c *connector.Client) ([]*pb.ProtoOAAsset, error) {
	session, err := c.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	accountID := session.CtidTraderAccountID
	req := &pb.ProtoOAAssetListReq{CtidTraderAccountId: &accountID}
	var res pb.ProtoOAAssetListRes
	if err := c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_ASSET_LIST_REQ, req, &res); err != nil {
		return nil, err
	}
	return res.GetAsset(), nil
}
