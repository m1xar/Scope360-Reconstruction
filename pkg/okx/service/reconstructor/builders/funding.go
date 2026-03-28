package builders

import (
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/service/reconstructor/helpers"
)

func BuildUserFunding(bill models.Bill) domain.UserFunding {
	return domain.UserFunding{
		Pair:      helpers.NormalizePair(bill.InstId),
		Amount:    helpers.Round8(helpers.MustFloat(bill.BalChg)),
		CreatedAt: helpers.TimeFromMs(bill.Ts),
	}
}
