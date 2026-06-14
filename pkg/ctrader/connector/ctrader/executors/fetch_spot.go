package executors

import (
	"context"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

func FetchSpot(ctx context.Context, c *connector.Client, symbolID int64) (*pb.ProtoOASpotEvent, error) {
	return c.FetchSpot(ctx, symbolID)
}
