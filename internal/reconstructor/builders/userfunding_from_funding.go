package builders

import (
	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/hyperliquid/models"
	"hyperliquid-trade-reconstructor/internal/reconstructor"
	"time"

	"github.com/google/uuid"
)

func BuildUserFunding(fund models.FundingHistoryItem) domain.UserFunding {

	return domain.UserFunding{
		UserID:    uuid.New(),
		KeyID:     0,
		Pair:      fund.Delta.Coin + "/USDC",
		Amount:    reconstructor.MustFloat(fund.Delta.USDC),
		CreatedAt: time.Unix(fund.Time, 0),
	}
}
