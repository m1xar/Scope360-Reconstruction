package builders

import (
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
	"time"
)

func BuildUserFunding(fund models.FundingHistoryItem) domain.UserFunding {

	return domain.UserFunding{
		Pair:      fund.Delta.Coin + "USDC",
		Amount:    helpers.Round8(helpers.MustFloat(fund.Delta.USDC)),
		CreatedAt: time.UnixMilli(fund.Time).UTC(),
	}
}
