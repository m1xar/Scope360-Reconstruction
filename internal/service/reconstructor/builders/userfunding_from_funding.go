package builders

import (
	"hyperliquid-trade-reconstructor/internal/connector/hyperliquid/models"
	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/service/reconstructor/helpers"
	"time"
)

func BuildUserFunding(fund models.FundingHistoryItem) domain.UserFunding {

	return domain.UserFunding{
		Pair:      fund.Delta.Coin + "USDC",
		Amount:    helpers.Round8(helpers.MustFloat(fund.Delta.USDC)),
		CreatedAt: time.UnixMilli(fund.Time).UTC(),
	}
}
