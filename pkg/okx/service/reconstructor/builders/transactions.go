package builders

import (
	"math"
	"sort"
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
)

const (
	okxFundingAccount = "6"
	okxTradingAccount = "18"
)

func BuildTransactionsFromBills(bills []models.Bill) []domain.Transaction {
	out := make([]domain.Transaction, 0, len(bills))
	for _, b := range bills {
		from := strings.TrimSpace(b.From)
		to := strings.TrimSpace(b.To)

		var typ string
		switch {
		case from == okxFundingAccount && to == okxTradingAccount:
			typ = domain.TransactionTypeDeposit
		case from == okxTradingAccount && to == okxFundingAccount:
			typ = domain.TransactionTypeWithdrawal
		default:
			continue
		}

		amount := helpers.Round8(math.Abs(helpers.MustFloat(b.BalChg)))
		if amount == 0 {
			amount = helpers.Round8(math.Abs(helpers.MustFloat(b.Sz)))
		}
		if amount == 0 {
			continue
		}

		out = append(out, domain.Transaction{
			Time:   helpers.TimeFromMs(b.Ts),
			Type:   typ,
			Amount: amount,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Time.Before(out[j].Time)
	})
	return out
}
