package builders

import (
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func BuildFundings(fees []models.FundingFee) []domain.UserFunding {
	fundings := make([]domain.UserFunding, 0, len(fees))

	for _, f := range fees {
		amount := helpers.MustFloat(f.FundingFee)
		if amount == 0 {
			continue
		}

		at := helpers.TimeFromMs(f.FundingTime)
		if at.UnixMilli() == 0 {
			at = helpers.TimeFromMs(f.Ts)
		}

		fundings = append(fundings, domain.UserFunding{
			Pair:      helpers.NormalizePair(f.InstID),
			Symbol:    f.InstID,
			Amount:    helpers.Round8(amount),
			CreatedAt: at,
		})
	}

	return fundings
}
