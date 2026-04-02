package reconstructor

import (
	"sort"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/workers"
)

const defaultCandleWorkers = 4

func ReconstructClosedPositions(client *resty.Client) ([]domain.Position, error) {
	closedPositions, err := executors.FetchAllHistoryPositions(client)
	if err != nil {
		return nil, err
	}
	if len(closedPositions) == 0 {
		return []domain.Position{}, nil
	}

	oldestMs := closedPositions[0].CreateTime
	for _, cp := range closedPositions[1:] {
		if cp.CreateTime < oldestMs {
			oldestMs = cp.CreateTime
		}
	}
	oldestMs -= 10 * 60 * 1000

	allOrders, err := executors.FetchAllHistoryOrders(client, oldestMs)
	if err != nil {
		return nil, err
	}
	ordersBySymbol := helpers.GroupOrdersBySymbol(allOrders)

	fundingRecords, err := executors.FetchAllFundingRecords(client)
	if err != nil {
		return nil, err
	}

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, candleRequests, defaultCandleWorkers)

	type pendingCandle struct {
		idx     int
		replyCh chan helpers.CandleResponse
	}

	pending := make([]pendingCandle, 0, len(closedPositions))
	positions := make([]domain.Position, len(closedPositions))

	for i, cp := range closedPositions {
		posOrders := helpers.MatchOrdersToPosition(cp, ordersBySymbol)

		funding := builders.ExtractFundingForPosition(
			fundingRecords, cp.Symbol, cp.CreateTime, cp.UpdateTime,
		)

		pos, err := builders.BuildPosition(cp, posOrders, funding)
		if err != nil {
			continue
		}
		positions[i] = pos

		replyCh := make(chan helpers.CandleResponse, 1)
		candleRequests <- helpers.CandleRequest{
			Symbol:  cp.Symbol,
			Bar:     "Min5",
			StartMs: cp.CreateTime,
			EndMs:   cp.UpdateTime,
			ReplyCh: replyCh,
		}
		pending = append(pending, pendingCandle{idx: i, replyCh: replyCh})
	}
	close(candleRequests)

	for _, p := range pending {
		resp := <-p.replyCh
		if resp.Err == nil {
			high, low := helpers.GetHighLow(resp.Candles)
			builders.ApplyMAEMFE(&positions[p.idx], high, low)
		}
	}

	filtered := make([]domain.Position, 0, len(positions))
	for _, pos := range positions {
		if pos.ID != uuid.Nil {
			filtered = append(filtered, pos)
		}
	}
	positions = filtered

	sort.Slice(positions, func(i, j int) bool {
		if positions[i].ClosedAt == nil {
			return true
		}
		if positions[j].ClosedAt == nil {
			return false
		}
		return positions[i].ClosedAt.Before(*positions[j].ClosedAt)
	})

	return positions, nil
}
