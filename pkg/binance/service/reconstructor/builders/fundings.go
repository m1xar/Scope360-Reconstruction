package builders

import (
	"github.com/m1xar/scope360-reconstruction/pkg/binance/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func BuildFundings(ledger *helpers.Ledger) []domain.UserFunding {
	fundings := make([]domain.UserFunding, 0, len(ledger.Fundings))

	for _, f := range ledger.Fundings {
		amount := helpers.MustFloat(f.Income)
		if amount == 0 {
			continue
		}

		fundings = append(fundings, domain.UserFunding{
			Pair:      helpers.NormalizePair(f.Symbol),
			Symbol:    f.Symbol,
			Amount:    helpers.Round8(amount),
			CreatedAt: helpers.TimeFromMs(f.Time),
		})
	}

	return fundings
}
