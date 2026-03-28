package builders

import (
	"sort"
	"strconv"
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
		ti := helpers.MustInt64(bills[i].Ts)
		tj := helpers.MustInt64(bills[j].Ts)
		if ti != tj {
			return ti > tj
		}

		bi, errI := strconv.ParseInt(bills[i].BillId, 10, 64)
		bj, errJ := strconv.ParseInt(bills[j].BillId, 10, 64)
		if errI == nil && errJ == nil {
			return bi > bj
		}
		return bills[i].BillId > bills[j].BillId
	})

	snapshots := make([]domain.UserBalanceSnapshot, 0, len(bills)+1)

	snapshots = append(snapshots, domain.UserBalanceSnapshot{
		CreatedAt: time.Now().UTC(),
		Balance:   helpers.Round8(currentBalance),
	})

	balance := currentBalance
	for _, bill := range bills {
		balance -= helpers.MustFloat(bill.BalChg)

		billTime := helpers.TimeFromMs(bill.Ts)
		roundedBalance := helpers.Round8(balance)

		lastIdx := len(snapshots) - 1
		if lastIdx >= 0 && snapshots[lastIdx].CreatedAt.Equal(billTime) {
			snapshots[lastIdx].Balance = roundedBalance
			continue
		}

		snapshots = append(snapshots, domain.UserBalanceSnapshot{
			CreatedAt: billTime,
			Balance:   roundedBalance,
		})
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})

	return snapshots
}
