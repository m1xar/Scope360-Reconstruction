package executors

import (
	"context"
	"errors"
	"strings"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

const (
	cashFlowWindow      = 7 * 24 * time.Hour
	cashFlowMaxAttempts = 3
	cashFlowRetryDelay  = 1500 * time.Millisecond
	cashFlowMaxDelay    = 2 * time.Second
)

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
		if err := fetchCashFlowChunk(ctx, c, req, &res); err != nil {
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

func fetchCashFlowChunk(
	ctx context.Context,
	c *connector.Client,
	req *pb.ProtoOACashFlowHistoryListReq,
	res *pb.ProtoOACashFlowHistoryListRes,
) error {
	var err error
	for attempt := 1; attempt <= cashFlowMaxAttempts; attempt++ {
		err = c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_CASH_FLOW_HISTORY_LIST_REQ, req, res)
		if err == nil || !isCashFlowRetryable(err) || attempt == cashFlowMaxAttempts {
			return err
		}
		if sleepErr := sleepCashFlowRetry(ctx, cashFlowDelay(err)); sleepErr != nil {
			return sleepErr
		}
	}
	return err
}

func isCashFlowRetryable(err error) bool {
	var apiErr connector.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == "BLOCKED_PAYLOAD_TYPE" || apiErr.Code == "TIMEOUT_ERROR"
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout")
}

func cashFlowDelay(err error) time.Duration {
	var apiErr connector.APIError
	if errors.As(err, &apiErr) && apiErr.RetryAfter > 0 {
		delay := time.Duration(apiErr.RetryAfter) * time.Second
		if delay < cashFlowMaxDelay {
			return delay
		}
		return cashFlowMaxDelay
	}
	return cashFlowRetryDelay
}

func sleepCashFlowRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
