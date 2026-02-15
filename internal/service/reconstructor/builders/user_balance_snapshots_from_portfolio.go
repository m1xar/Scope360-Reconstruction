package builders

import (
	"hyperliquid-trade-reconstructor/internal/connector/hyperliquid/models"
	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/service/reconstructor/helpers"
	"strings"
	"time"
)

func BuildUserBalanceSnapshotsFromPortfolio(resp models.PortfolioResponse) []domain.UserBalanceSnapshot {
	history := extractAllTimeAccountValueHistory(resp)
	out := make([]domain.UserBalanceSnapshot, 0, len(history))

	for _, point := range history {
		out = append(out, domain.UserBalanceSnapshot{
			ResourceID: 0,
			CreatedAt:  time.UnixMilli(point.Timestamp).UTC(),
			Balance:    helpers.Round8(helpers.MustFloat(point.Value)),
		})
	}

	return out
}

func extractAllTimeAccountValueHistory(resp models.PortfolioResponse) []models.HistoryPoint {
	for _, entry := range resp {
		if strings.EqualFold(entry.Period, "allTime") {
			return entry.Data.AccountValueHistory
		}
	}

	return []models.HistoryPoint{}
}
