package builders

import (
	"sort"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/helpers"
)

var stableBalanceAssets = map[string]struct{}{
	"USD":  {},
	"USDT": {},
	"USDC": {},
}

func BuildBalanceSnapshots(logs []models.AccountLog) []domain.UserBalanceSnapshot {
	type balanceRow struct {
		row models.AccountLog
		at  time.Time
	}

	rows := make([]balanceRow, 0, len(logs))
	for _, row := range logs {
		asset := strings.ToUpper(strings.TrimSpace(row.Asset))
		if _, ok := stableBalanceAssets[asset]; !ok || !row.NewBalance.Valid {
			continue
		}
		at, err := helpers.ParseTime(row.Date)
		if err != nil {
			continue
		}
		rows = append(rows, balanceRow{row: row, at: at})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].at.Equal(rows[j].at) {
			return rows[i].row.ID < rows[j].row.ID
		}
		return rows[i].at.Before(rows[j].at)
	})

	balances := make(map[string]float64, len(stableBalanceAssets))
	out := make([]domain.UserBalanceSnapshot, 0, len(rows))
	for _, item := range rows {
		asset := strings.ToUpper(strings.TrimSpace(item.row.Asset))
		balances[asset] = item.row.NewBalance.Value

		var total float64
		for stable := range stableBalanceAssets {
			total += balances[stable]
		}

		out = append(out, domain.UserBalanceSnapshot{
			ResourceID: uint64(item.row.ID),
			CreatedAt:  item.at,
			Balance:    helpers.Round8(total),
		})
	}

	return out
}
