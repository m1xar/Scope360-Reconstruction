package reconstructor

import (
	"sort"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/workers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

const defaultCandleWorkers = 4

func ReconstructClosedPositions(client *resty.Client) ([]domain.Position, error) {
	rawPositions, err := executors.FetchAllPositionHistory(client)
	if err != nil {
		return nil, err
	}
	if len(rawPositions) == 0 {
		return []domain.Position{}, nil
	}

	instruments, err := executors.FetchAllInstruments(client)
	if err != nil {
		return nil, err
	}
	contractValues := helpers.ContractValueByInstID(instruments)

	oldestCreateTime := helpers.MustInt64(rawPositions[0].CreateTime)
	for _, pos := range rawPositions[1:] {
		if t := helpers.MustInt64(pos.CreateTime); t > 0 && t < oldestCreateTime {
			oldestCreateTime = t
		}
	}
	oldestMs := oldestCreateTime - 10*60*1000
	if oldestMs < 0 {
		oldestMs = 0
	}

	orders, err := executors.FetchAllOrdersHistory(client, oldestMs)
	if err != nil {
		return nil, err
	}
	tpslOrders, err := executors.FetchAllTPSLOrdersHistory(client, oldestMs)
	if err != nil {
		return nil, err
	}
	algoOrders, err := executors.FetchAllAlgoOrdersHistory(client, oldestMs)
	if err != nil {
		return nil, err
	}

	ordersByInst := helpers.GroupOrdersByInst(orders)
	tpslByInst := helpers.GroupOrdersByInst(tpslOrders)
	algoByInst := helpers.GroupOrdersByInst(algoOrders)

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, candleRequests, defaultCandleWorkers)

	type pendingCandle struct {
		idx     int
		replyCh chan helpers.CandleResponse
	}

	pending := make([]pendingCandle, 0, len(rawPositions))
	positions := make([]domain.Position, len(rawPositions))
	for i, raw := range rawPositions {
		posOrders := helpers.MatchOrdersToPosition(raw, ordersByInst)
		posTPSL := helpers.MatchOrdersToPosition(raw, tpslByInst)
		posAlgo := helpers.MatchOrdersToPosition(raw, algoByInst)

		pos, err := builders.BuildPosition(raw, posOrders, posTPSL, posAlgo, contractValues[raw.InstID])
		if err != nil {
			continue
		}
		positions[i] = pos

		startMs, endMs := helpers.PositionTimeMs(raw)
		if startMs == 0 || endMs == 0 || endMs < startMs {
			continue
		}
		replyCh := make(chan helpers.CandleResponse, 1)
		candleRequests <- helpers.CandleRequest{
			InstID:  raw.InstID,
			Bar:     "1m",
			StartMs: startMs,
			EndMs:   endMs,
			ReplyCh: replyCh,
		}
		pending = append(pending, pendingCandle{idx: i, replyCh: replyCh})
	}
	close(candleRequests)

	for _, p := range pending {
		resp := <-p.replyCh
		if resp.Err != nil {
			continue
		}
		high, low := helpers.GetHighLow(resp.Candles)
		builders.ApplyMAEMFE(&positions[p.idx], high, low)
	}

	filtered := make([]domain.Position, 0, len(positions))
	for _, pos := range positions {
		if pos.ID != uuid.Nil {
			filtered = append(filtered, pos)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].ClosedAt == nil || filtered[j].ClosedAt == nil {
			return filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
		}
		return filtered[i].ClosedAt.Before(*filtered[j].ClosedAt)
	})

	return filtered, nil
}
