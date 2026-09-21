package reconstructor

import (
	"sort"
	"strings"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/workers"
)

const defaultCandleWorkers = 4

// orders-history-archive filters begin/end by order cTime, and a resting limit
// order is created before the position it opens, so the fetch window has to
// start earlier than the position's cTime.
const openPositionOrdersLookback = 7 * 24 * 60 * 60 * 1000

func ReconstructClosedPositions(
	client *resty.Client,
	baseURL string,
	days int,
) ([]domain.Position, error) {
	closedPositions, err := executors.FetchAllClosedPositions(client, baseURL)
	if err != nil {
		return nil, err
	}
	if len(closedPositions) == 0 {
		return []domain.Position{}, nil
	}

	oldestMs := helpers.MustInt64(closedPositions[0].CTime)
	for _, cp := range closedPositions[1:] {
		if t := helpers.MustInt64(cp.CTime); t < oldestMs {
			oldestMs = t
		}
	}

	oldestMs -= 10 * 60 * 1000

	allOrders, fills, fillsErr, err := fetchOrdersAndFills(client, baseURL, oldestMs, oldestMs)
	if err != nil {
		return nil, err
	}
	ordersByInst := helpers.GroupOrdersByInst(allOrders)
	parents := helpers.OrdersByID(allOrders)
	fillsByInst := helpers.GroupFillsByInst(fills)

	candleRequests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, baseURL, candleRequests, defaultCandleWorkers)

	type pendingCandle struct {
		idx     int
		replyCh chan helpers.CandleResponse
	}

	identifiers := map[string]models.Instrumentidentifier{}
	for _, cp := range closedPositions {
		_, ok := identifiers[cp.InstId]
		if ok {
			continue
		}

		identifiers[cp.InstId] = models.Instrumentidentifier{InstID: cp.InstId, InstType: cp.InstType}
	}

	instruments, err := executors.FetchInstruments(client, baseURL, identifiers)
	if err != nil {
		return nil, err
	}

	pending := make([]pendingCandle, 0, len(closedPositions))
	positions := make([]domain.Position, len(closedPositions))

	for i, cp := range closedPositions {
		var posOrders []models.Order
		if fillsErr == nil {
			posOrders = helpers.OrdersFromFills(fillsByInst[cp.InstId], parents, helpers.PositionScope{
				InstId:  cp.InstId,
				PosSide: cp.Direction,
				MgnMode: cp.MgnMode,
				FromMs:  helpers.MustInt64(cp.CTime),
				ToMs:    helpers.MustInt64(cp.UTime),
			}, instruments[cp.InstId])
		}
		if len(posOrders) == 0 {
			posOrders = helpers.MatchOrdersToPosition(cp, ordersByInst, instruments[cp.InstId])
		}
		pos, err := helpers.BuildPosition(cp, posOrders, instruments[cp.InstId])
		if err != nil {
			continue
		}
		positions[i] = pos

		replyCh := make(chan helpers.CandleResponse, 1)
		candleRequests <- helpers.CandleRequest{
			InstId:  cp.InstId,
			Bar:     "1m",
			StartMs: helpers.MustInt64(cp.CTime),
			EndMs:   helpers.MustInt64(cp.UTime),
			ReplyCh: replyCh,
		}
		pending = append(pending, pendingCandle{idx: i, replyCh: replyCh})
	}
	close(candleRequests)

	for _, p := range pending {
		resp := <-p.replyCh
		if resp.Err == nil {
			high, low := helpers.GetHighLow(resp.Candles)
			helpers.ApplyMAEMFE(&positions[p.idx], high, low)
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
		return positions[i].ClosedAt.Before(*positions[j].ClosedAt)
	})

	cutoff := helpers.CutoffFromDays(days)
	if cutoff != nil {
		trimmed := positions[:0]
		for _, pos := range positions {
			if pos.ClosedAt != nil && !pos.ClosedAt.Before(*cutoff) {
				trimmed = append(trimmed, pos)
			}
		}
		positions = trimmed
	}

	balance, err := executors.FetchBalance(client, baseURL)
	if err == nil {
		currentBal := helpers.MustFloat(balance.TotalEq)
		bills, billsErr := executors.FetchAllSwapAndFuturesBills(client, baseURL, oldestMs, "")
		if billsErr == nil && len(bills) > 0 {
			snapshots := builders.BuildBalanceSnapshotsFromBills(currentBal, bills)
			helpers.AttachBalanceInit(&positions, snapshots)
		}
	}

	return positions, nil
}

func ReconstructOpenPositions(
	client *resty.Client,
	baseURL string,
) ([]domain.OpenPosition, error) {
	raw, err := executors.FetchOpenPositions(client, baseURL)
	if err != nil {
		return nil, err
	}

	identifiers := map[string]models.Instrumentidentifier{}
	for _, cp := range raw {
		_, ok := identifiers[cp.InstId]
		if ok {
			continue
		}

		identifiers[cp.InstId] = models.Instrumentidentifier{InstID: cp.InstId, InstType: cp.InstType}
	}

	instruments, err := executors.FetchInstruments(client, baseURL, identifiers)
	if err != nil {
		return nil, err
	}

	positions := make([]domain.OpenPosition, 0, len(raw))
	for _, r := range raw {
		positions = append(positions, builders.BuildOpenPosition(r, instruments[r.InstId]))
	}

	enrichOpenPositionOrders(client, baseURL, raw, positions, instruments)
	return positions, nil
}

func enrichOpenPositionOrders(
	client *resty.Client,
	baseURL string,
	raw []models.OpenPosition,
	positions []domain.OpenPosition,
	instruments map[string]models.Instrument,
) {
	if len(raw) == 0 || len(positions) == 0 {
		return
	}

	startMs := int64(0)
	for _, r := range raw {
		t := helpers.MustInt64(r.CTime)
		if t == 0 {
			continue
		}
		if startMs == 0 || t < startMs {
			startMs = t
		}
	}
	if startMs == 0 {
		return
	}

	orders, fills, fillsErr, err := fetchOrdersAndFills(client, baseURL, startMs-openPositionOrdersLookback, startMs)
	if err != nil {
		return
	}
	parents := helpers.OrdersByID(orders)
	fillsByInst := helpers.GroupFillsByInst(fills)

	for i := range positions {
		if i >= len(raw) {
			return
		}
		r := raw[i]
		openMs := helpers.MustInt64(r.CTime)
		if fillsErr == nil {
			fromFills := helpers.OrdersFromFills(fillsByInst[r.InstId], parents, helpers.PositionScope{
				InstId:  r.InstId,
				PosSide: r.PosSide,
				MgnMode: r.MgnMode,
				FromMs:  openMs,
			}, instruments[r.InstId])
			if len(fromFills) > 0 {
				positions[i].Orders = helpers.BuildOrders(fromFills, positions[i].ID)
				continue
			}
		}
		posSide := strings.ToLower(strings.TrimSpace(r.PosSide))
		mgnMode := strings.ToLower(strings.TrimSpace(r.MgnMode))
		matched := make([]models.Order, 0)

		for _, ord := range orders {
			if ord.InstId != r.InstId {
				continue
			}
			ordPosSide := strings.ToLower(strings.TrimSpace(ord.PosSide))
			if posSide != "" && posSide != "net" && ordPosSide != "" && ordPosSide != "net" && ordPosSide != posSide {
				continue
			}
			ordTdMode := strings.ToLower(strings.TrimSpace(ord.TdMode))
			if mgnMode != "" && ordTdMode != "" && ordTdMode != mgnMode {
				continue
			}
			if helpers.MustInt64(ord.UTime) < openMs {
				continue
			}
			matched = append(matched, helpers.OrderInBaseUnits(ord, instruments[r.InstId]))
		}

		positions[i].Orders = helpers.BuildOrders(matched, positions[i].ID)
	}
}

// fetchOrdersAndFills loads order history (by order cTime, from ordersFromMs)
// and fill history (by fill ts, from fillsFromMs) in parallel. Orders are
// required; a fills failure is returned separately so callers can fall back
// to whole-order matching.
func fetchOrdersAndFills(
	client *resty.Client,
	baseURL string,
	ordersFromMs, fillsFromMs int64,
) (orders []models.Order, fills []models.Fill, fillsErr, err error) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		orders, err = executors.FetchAllSwapAndFuturesOrders(client, baseURL, ordersFromMs)
	}()
	go func() {
		defer wg.Done()
		fills, fillsErr = executors.FetchAllSwapAndFuturesFills(client, baseURL, fillsFromMs)
	}()
	wg.Wait()
	return orders, fills, fillsErr, err
}
