package builders

import (
	"math"
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
)

func BuildTransactions(transfers []models.CashTransfer) []domain.Transaction {
	var out []domain.Transaction
	for _, t := range transfers {
		for _, tx := range t.CashTransactions {
			var kind string
			switch tx.Type {
			case models.CashDeposit:
				kind = domain.TransactionTypeDeposit
			case models.CashWithdrawal:
				kind = domain.TransactionTypeWithdrawal
			default:
				continue
			}
			at := tx.TransactionTime.Time
			if at.IsZero() {
				at = t.TransactionTime.Time
			}
			out = append(out, domain.Transaction{Time: at, Type: kind, Amount: math.Abs(tx.Value.Float())})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out
}
