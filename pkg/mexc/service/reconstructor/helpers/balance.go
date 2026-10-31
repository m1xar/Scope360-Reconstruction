package helpers

import (
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func AttachBalanceInit(positions *[]domain.Position, snapshots []domain.UserBalanceSnapshot) {
	if positions == nil || len(*positions) == 0 || len(snapshots) == 0 {
		return
	}

	sorted := make([]domain.UserBalanceSnapshot, len(snapshots))
	copy(sorted, snapshots)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
	})

	ps := *positions
	order := make([]int, len(ps))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		pa, pb := ps[order[a]], ps[order[b]]
		if pa.ClosedAt == nil || pb.ClosedAt == nil {
			return pa.ClosedAt != nil && pb.ClosedAt == nil
		}
		return pa.ClosedAt.Before(*pb.ClosedAt)
	})

	closed := make([]int, 0, len(ps))
	for _, i := range order {
		if ps[i].ClosedAt != nil {
			closed = append(closed, i)
		}
	}

	for _, i := range order {
		pos := &ps[i]
		pos.BalanceInit = 0
		idx := sort.Search(len(sorted), func(k int) bool {
			return sorted[k].CreatedAt.UnixNano() > pos.CreatedAt.UnixNano()
		}) - 1
		if idx >= 0 {
			pos.BalanceInit = Round8(sorted[idx].Balance)
		}
		if pos.BalanceInit >= 1 {
			continue
		}
		pos.BalanceInit = 0
		prev := sort.Search(len(closed), func(k int) bool {
			return !ps[closed[k]].ClosedAt.Before(pos.CreatedAt)
		}) - 1
		if prev >= 0 {
			p := ps[closed[prev]]
			pos.BalanceInit = Round8(max(p.BalanceInit+p.NetPnl, 0))
		}
	}
}
