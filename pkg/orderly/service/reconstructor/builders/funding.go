package builders

import (
	"time"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/service/reconstructor/helpers"
)

func BuildUserFunding(fund models.OrderlyFunding) domain.UserFunding {
	return domain.UserFunding{
		Pair:      helpers.NormalizeSymbol(fund.Symbol),
		Amount:    helpers.Round8(fund.FundingFee),
		CreatedAt: time.UnixMilli(fund.CreatedTime).UTC(),
	}
}
