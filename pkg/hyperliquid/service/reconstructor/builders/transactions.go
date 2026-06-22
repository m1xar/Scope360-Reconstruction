package builders

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
)

func BuildTransactions(updates []models.NonFundingLedgerUpdate) []domain.Transaction {
	out := make([]domain.Transaction, 0, len(updates))
	for _, update := range updates {
		var typ string
		switch strings.ToLower(strings.TrimSpace(update.Delta.Type)) {
		case "deposit":
			typ = domain.TransactionTypeDeposit
		case "withdraw":
			typ = domain.TransactionTypeWithdrawal
		case "accountclasstransfer":
			if update.Delta.ToPerp == nil {
				continue
			}
			if *update.Delta.ToPerp {
				typ = domain.TransactionTypeDeposit
			} else {
				typ = domain.TransactionTypeWithdrawal
			}
		case "send":
			sourceDex := strings.ToLower(strings.TrimSpace(update.Delta.SourceDex))
			destinationDex := strings.ToLower(strings.TrimSpace(update.Delta.DestinationDex))
			switch {
			case sourceDex == "spot" && destinationDex == "":
				typ = domain.TransactionTypeDeposit
			case sourceDex == "" && destinationDex == "spot":
				typ = domain.TransactionTypeWithdrawal
			default:
				continue
			}
		default:
			continue
		}

		amount := helpers.Round8(math.Abs(helpers.MustFloat(transactionAmount(update.Delta))))
		if amount == 0 {
			continue
		}
		out = append(out, domain.Transaction{
			Time:   time.UnixMilli(update.Time).UTC(),
			Type:   typ,
			Amount: amount,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Time.Before(out[j].Time)
	})
	return out
}

func transactionAmount(delta models.NonFundingLedgerDelta) string {
	if strings.TrimSpace(delta.USDC) != "" {
		return delta.USDC
	}
	if strings.TrimSpace(delta.USDCValue) != "" {
		return delta.USDCValue
	}
	if strings.EqualFold(delta.Token, "USDC") {
		return delta.Amount
	}
	return ""
}
