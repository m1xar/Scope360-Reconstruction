package builders

import (
	"sort"
	"time"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/service/reconstructor/helpers"
)

func BuildBalanceSnapshots(currentBalance float64, bills []models.Bill) []domain.UserBalanceSnapshot {
	if len(bills) == 0 {
		return []domain.UserBalanceSnapshot{{
			CreatedAt: time.Now().UTC(),
			Balance:   helpers.Round8(currentBalance),
		}}
	}

	sort.Slice(bills, func(i, j int) bool {
		return helpers.MustInt64(bills[i].Ts) > helpers.MustInt64(bills[j].Ts)
	})

	snapshots := make([]domain.UserBalanceSnapshot, 0, len(bills)+1)

	snapshots = append(snapshots, domain.UserBalanceSnapshot{
		CreatedAt: time.Now().UTC(),
		Balance:   helpers.Round8(currentBalance),
	})

	balance := currentBalance
	for _, bill := range bills {
		balance -= helpers.MustFloat(bill.BalChg)
		snapshots = append(snapshots, domain.UserBalanceSnapshot{
			CreatedAt: helpers.TimeFromMs(bill.Ts),
			Balance:   helpers.Round8(balance),
		})
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})

	return snapshots
}
