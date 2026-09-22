package reconstructor

import (
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/builders"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/scope"
)

const defaultCandleWorkers = 4

// ReconstructClosedPositions builds the positions closed after the cutoff
// (the whole retention when cutoff is nil).
func ReconstructClosedPositions(client *resty.Client, cutoff *time.Time) ([]domain.Position, error) {
	d, err := Load(client, cutoff, scope.Closed)
	if err != nil {
		return nil, err
	}
	return d.ClosedPositions()
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

// enrichOpenPositionOrders attaches each open position's orders, matched
// by the exchange's position id.
func enrichOpenPositionOrders(
	orders []models.Order,
	raw []models.OpenPosition,
	positions []domain.OpenPosition,
) {
	if len(raw) == 0 || len(positions) == 0 {
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
