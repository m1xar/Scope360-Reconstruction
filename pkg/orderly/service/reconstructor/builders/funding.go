package builders

import (
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly/models"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/service/reconstructor/helpers"
)

func BuildUserFunding(fund models.OrderlyFunding) domain.UserFunding {
	return domain.UserFunding{
		Pair:      helpers.NormalizeSymbol(fund.Symbol),
		Amount:    helpers.Round8(fund.FundingFee),
		CreatedAt: time.UnixMilli(fund.CreatedTime).UTC(),
	}
}
