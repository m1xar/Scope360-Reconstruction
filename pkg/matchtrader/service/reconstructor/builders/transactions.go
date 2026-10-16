package builders

import (
	"math"
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

func BuildTransactions(finance []models.FinanceOperation, synthetic *fx.BalanceEvent) []domain.Transaction {
	out := make([]domain.Transaction, 0, len(finance)+1)
	for _, op := range finance {
		var typ string
		switch op.Kind() {
		case models.FinanceDeposit:
			typ = domain.TransactionTypeDeposit
		case models.FinanceWithdrawal:
			typ = domain.TransactionTypeWithdrawal
		default:
			continue
		}
		amount := fx.Round8(math.Abs(op.Profit.Float()))
		if amount == 0 {
			continue
		}
		out = append(out, domain.Transaction{Time: op.At(), Type: typ, Amount: amount})
	}
	if synthetic != nil && synthetic.Delta > 0 {
		out = append(out, domain.Transaction{Time: synthetic.Time, Type: domain.TransactionTypeDeposit, Amount: fx.Round8(synthetic.Delta)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out
}
