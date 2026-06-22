package builders

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/models"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/reconstructor/helpers"
)

func BuildTransactions(assetHistory []models.OrderlyAssetHistory, markPrices map[string]float64) ([]domain.Transaction, error) {
	out := make([]domain.Transaction, 0, len(assetHistory))
	for _, ev := range assetHistory {
		if !isCompletedTransfer(ev) {
			continue
		}

		var typ string
		switch strings.ToUpper(strings.TrimSpace(ev.Side)) {
		case "DEPOSIT":
			typ = domain.TransactionTypeDeposit
		case "WITHDRAW", "WITHDRAWAL":
			typ = domain.TransactionTypeWithdrawal
		default:
			continue
		}

		price := tokenUSDCPrice(ev.Token, markPrices)
		if price <= 0 {
			return nil, fmt.Errorf("missing USDC mark price for token %q", ev.Token)
		}

		amount := helpers.Round8(math.Abs(ev.Amount) * price)
		if amount == 0 {
			continue
		}
		out = append(out, domain.Transaction{
			Time:   time.UnixMilli(ev.CreatedTime).UTC(),
			Type:   typ,
			Amount: amount,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Time.Before(out[j].Time)
	})
	return out, nil
}
