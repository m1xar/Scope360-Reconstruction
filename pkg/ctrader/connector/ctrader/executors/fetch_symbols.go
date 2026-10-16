package executors

import (
	"context"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

func FetchLightSymbols(ctx context.Context, c *connector.Client) ([]*pb.ProtoOALightSymbol, error) {
	session, err := c.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	accountID := session.CtidTraderAccountID
	req := &pb.ProtoOASymbolsListReq{CtidTraderAccountId: &accountID}
	var res pb.ProtoOASymbolsListRes
	if err := c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_SYMBOLS_LIST_REQ, req, &res); err != nil {
		return nil, err
	}
	return res.GetSymbol(), nil
}

func FetchSymbolNames(ctx context.Context, c *connector.Client) (map[int64]string, error) {
	items, err := FetchLightSymbols(ctx, c)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(items))
	for _, symbol := range items {
		out[symbol.GetSymbolId()] = symbol.GetSymbolName()
	}
	return out, nil
}

func FetchSymbols(ctx context.Context, c *connector.Client, ids []int64) ([]*pb.ProtoOASymbol, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	session, err := c.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	accountID := session.CtidTraderAccountID
	req := &pb.ProtoOASymbolByIdReq{CtidTraderAccountId: &accountID, SymbolId: ids}
	var res pb.ProtoOASymbolByIdRes
	if err := c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_SYMBOL_BY_ID_REQ, req, &res); err != nil {
		return nil, err
	}
	return res.GetSymbol(), nil
}
