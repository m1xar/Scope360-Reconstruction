package builders

import (
	"math"
	"sort"
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
)

func BuildTransactions(transfers []models.TransferRecord) []domain.Transaction {
	out := make([]domain.Transaction, 0, len(transfers))
	for _, tr := range transfers {
		if !strings.EqualFold(tr.State, "SUCCESS") {
			continue
		}

		var typ string
		switch strings.ToUpper(strings.TrimSpace(tr.Type)) {
		case "IN":
			typ = domain.TransactionTypeDeposit
		case "OUT":
			typ = domain.TransactionTypeWithdrawal
		default:
			continue
		}

		amount := helpers.Round8(math.Abs(tr.Amount))
		if amount == 0 {
			continue
		}
		out = append(out, domain.Transaction{
			Time:   helpers.TimeFromMs(tr.CreateTime),
			Type:   typ,
			Amount: amount,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Time.Before(out[j].Time)
	})
	return out
}
