package builders

import (
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func BuildBalanceSnapshots(positions []domain.FXPosition) []domain.UserBalanceSnapshot {
	closed := append([]domain.FXPosition(nil), positions...)
	sort.Slice(closed, func(i, j int) bool {
		if closed[i].ClosedAt == nil {
			return true
		}
		if closed[j].ClosedAt == nil {
			return false
		}
		return closed[i].ClosedAt.Before(*closed[j].ClosedAt)
	})
	if len(closed) == 0 {
		return []domain.UserBalanceSnapshot{}
	}
	snapshots := make([]domain.UserBalanceSnapshot, 0, len(closed)+1)
	snapshots = append(snapshots, domain.UserBalanceSnapshot{ResourceID: 0, CreatedAt: closed[0].CreatedAt, Balance: closed[0].BalanceInit})
	for _, pos := range closed {
		if pos.ClosedAt == nil {
			continue
		}
		snapshots = append(snapshots, domain.UserBalanceSnapshot{
			ResourceID: uint64(pos.ID.ID()),
			CreatedAt:  *pos.ClosedAt,
			Balance:    pos.BalanceInit + pos.NetPnl,
		})
	}
	return snapshots
}
