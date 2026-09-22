package reconstructor

import (
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/workers"
)

const defaultCandleWorkers = 4

func ReconstructClosedPositions(client *resty.Client, cutoff *time.Time) ([]domain.Position, error) {
	cutoffMs := int64(0)
	if cutoff != nil {
		cutoffMs = cutoff.UnixMilli()
	}
	closedPositions, err := executors.FetchAllHistoryPositions(client, cutoffMs)
	if err != nil {
		return nil, err
	}

	closedPositions = positionsClosedAfter(closedPositions, cutoff)
	if len(closedPositions) == 0 {
		return []domain.Position{}, nil
	}
	contractSizes := fetchContractSizes(client, closedPositions)

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

	fundingRecords, err := executors.FetchAllFundingRecords(client, oldestMs)
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

		pos, err := builders.BuildPosition(cp, posOrders, funding, contractSizes[cp.Symbol])
		if err != nil {
			continue
		}
		positions[i] = pos

		replyCh := make(chan helpers.CandleResponse, 1)
		candleRequests <- helpers.CandleRequest{
			Symbol:  cp.Symbol,
			Bar:     "1m",
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

func positionsClosedAfter(positions []models.HistoryPosition, cutoff *time.Time) []models.HistoryPosition {
	if cutoff == nil {
		return positions
	}

	cutoffMs := cutoff.UnixMilli()
	kept := make([]models.HistoryPosition, 0, len(positions))
	for _, pos := range positions {
		if pos.UpdateTime < cutoffMs {
			continue
		}
		kept = append(kept, pos)
	}
	return kept
}

func fetchContractSizes(client *resty.Client, positions []models.HistoryPosition) map[string]float64 {
	sizes := make(map[string]float64)

	details, err := executors.FetchAllContractDetails(client)
	if err == nil {
		for _, detail := range details {
			if detail.Symbol != "" && detail.ContractSize > 0 {
				sizes[detail.Symbol] = detail.ContractSize
			}
		}
	}

	for _, pos := range positions {
		if pos.Symbol == "" || sizes[pos.Symbol] > 0 {
			continue
		}
		detail, err := executors.FetchContractDetail(client, pos.Symbol)
		if err != nil || detail.ContractSize <= 0 {
			continue
		}
		sizes[pos.Symbol] = detail.ContractSize
	}

	return sizes
}

func FetchStableEquity(client *resty.Client) (float64, error) {
	assets, err := executors.FetchAssets(client)
	if err == nil {
		var total float64
		var found bool
		for _, asset := range assets {
			if helpers.IsStableCurrency(asset.Currency) {
				total += asset.Equity
				found = true
			}
		}
		if found {
			return helpers.Round8(total), nil
		}
	}

	asset, fallbackErr := executors.FetchUSDTAsset(client)
	if fallbackErr != nil {
		if err != nil {
			return 0, err
		}
		return 0, fallbackErr
	}
	return helpers.Round8(asset.Equity), nil
}

func BalanceSnapshots(
	client *resty.Client,
	positions []domain.Position,
	cutoff *time.Time,
) ([]domain.UserBalanceSnapshot, error) {
	currentEquity, err := FetchStableEquity(client)
	if err != nil {
		return nil, err
	}

	windowStart := helpers.BalanceWindowStart(positions, cutoff)
	transfersSinceMs := int64(0)
	if windowStart != nil {
		transfersSinceMs = windowStart.UnixMilli()
	}
	transfers, err := executors.FetchAllTransferRecords(client, transfersSinceMs)
	if err != nil {
		return nil, err
	}

	return builders.BuildBalanceSnapshots(
		currentEquity,
		transfers,
		positions,
		windowStart,
	), nil
}

func EnrichOpenPositionOrders(
	client *resty.Client,
	raw []models.OpenPosition,
	positions []domain.OpenPosition,
) {
	if len(raw) == 0 || len(positions) == 0 {
		return
	}

	startMs := int64(0)
	for _, r := range raw {
		if r.HoldVol <= 0 {
			continue
		}
		if startMs == 0 || r.CreateTime < startMs {
			startMs = r.CreateTime
		}
	}
	if startMs == 0 {
		return
	}

	orders, err := executors.FetchAllHistoryOrders(client, startMs)
	if err != nil {
		return
	}

	byPositionID := make(map[int64][]models.Order)
	for _, ord := range orders {
		byPositionID[ord.PositionId] = append(byPositionID[ord.PositionId], ord)
	}

	posIdx := 0
	for _, r := range raw {
		if r.HoldVol <= 0 {
			continue
		}
		if posIdx >= len(positions) {
			return
		}
		positions[posIdx].Orders = builders.BuildOrders(byPositionID[r.PositionId], positions[posIdx].ID)
		posIdx++
	}
}
