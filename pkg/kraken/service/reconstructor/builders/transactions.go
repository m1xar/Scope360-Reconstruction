package builders

import (
	"math"
	"sort"
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/helpers"
)

func BuildTransactions(logs []models.AccountLog) []domain.Transaction {
	out := make([]domain.Transaction, 0, len(logs))
	for _, row := range logs {
		if !isFuturesAccountTransfer(row) || !row.OldBalance.Valid || !row.NewBalance.Valid {
			continue
		}

		delta := helpers.Round8(row.NewBalance.Value - row.OldBalance.Value)
		if delta == 0 {
			continue
		}

		at, err := helpers.ParseTime(row.Date)
		if err != nil {
			continue
		}

		typ := domain.TransactionTypeDeposit
		if delta < 0 {
			typ = domain.TransactionTypeWithdrawal
		}

		out = append(out, domain.Transaction{
			Time:   at,
			Type:   typ,
			Amount: helpers.Round8(math.Abs(delta)),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Time.Before(out[j].Time)
	})
	return out
}

func isFuturesAccountTransfer(row models.AccountLog) bool {
	info := strings.ToLower(strings.TrimSpace(row.Info))
	if !strings.Contains(info, "transfer") {
		return false
	}
	return row.RealizedPnL.Valid == false &&
		row.RealizedFunding.Valid == false &&
		(!row.Fee.Valid || row.Fee.Value == 0)
}
