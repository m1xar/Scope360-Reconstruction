package builders

import (
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
)

func BuildUserFundings(records []models.FundingRecord) []domain.UserFunding {
	result := make([]domain.UserFunding, 0, len(records))
	for _, r := range records {
		if r.Funding == 0 {
			continue
		}
		result = append(result, domain.UserFunding{
			Pair:      helpers.NormalizePair(r.Symbol),
			Amount:    helpers.Round8(r.Funding),
			CreatedAt: helpers.TimeFromMs(r.SettleTime),
		})
	}
	return result
}

func ExtractFundingForPosition(
	records []models.FundingRecord,
	symbol string,
	startMs, endMs int64,
) float64 {
	var total float64
	for _, r := range records {
		if r.Symbol != symbol {
			continue
		}
		if r.SettleTime >= startMs && r.SettleTime <= endMs {
			total += r.Funding
		}
	}
	return total
}
