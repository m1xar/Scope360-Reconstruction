package helpers

import (
	"fmt"
	"sort"
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
)

// boundarySlackMs tolerates OKX stamping a position's cTime/uTime a few ms off
// its first/last fill ts (seen: the closing fill 1ms after uTime). Only fills
// that open (before FromMs) or close (after ToMs) the position are admitted in
// the slack, so neighbouring positions on the same side are not picked up.
const boundarySlackMs = 1000

// PositionScope identifies the fills that belong to one position: OKX sets a
// position's cTime and uTime to the ts of its first and last fill. ToMs <= 0
// means the position is still open.
type PositionScope struct {
	InstId  string
	PosSide string
	MgnMode string
	FromMs  int64
	ToMs    int64
}

func GroupFillsByInst(fills []models.Fill) map[string][]models.Fill {
	idx := make(map[string][]models.Fill)
	for _, f := range fills {
		idx[f.InstId] = append(idx[f.InstId], f)
	}
	return idx
}

func OrdersByID(orders []models.Order) map[string]models.Order {
	idx := make(map[string]models.Order, len(orders))
	for _, o := range orders {
		idx[o.OrdId] = o
	}
	return idx
}

// OrdersFromFills rebuilds a position's orders from its fills, so an order
// whose fills are split between two positions contributes to each only the
// part it filled there. Sizes are returned in base-asset units.
func OrdersFromFills(
	fills []models.Fill,
	parents map[string]models.Order,
	scope PositionScope,
	instrument models.Instrument,
) []models.Order {
	posSide := strings.ToLower(scope.PosSide)
	mgnMode := strings.ToLower(scope.MgnMode)

	type agg struct {
		first           models.Fill
		size, notional  float64
		fee, pnl        float64
		firstMs, lastMs int64
	}
	byOrder := make(map[string]*agg)
	var ids []string

	for _, f := range fills {
		if f.InstId != scope.InstId {
			continue
		}
		ts := MustInt64(f.Ts)
		if !scope.covers(ts, strings.ToLower(f.Side)) {
			continue
		}
		fillPosSide := strings.ToLower(f.PosSide)
		if posSide != "" && posSide != "net" && fillPosSide != "" && fillPosSide != "net" && fillPosSide != posSide {
			continue
		}
		if parent, ok := parents[f.OrdId]; ok && mgnMode != "" && parent.TdMode != "" && strings.ToLower(parent.TdMode) != mgnMode {
			continue
		}

		a := byOrder[f.OrdId]
		if a == nil {
			a = &agg{first: f, firstMs: ts, lastMs: ts}
			byOrder[f.OrdId] = a
			ids = append(ids, f.OrdId)
		}
		sz := MustFloat(f.FillSz)
		a.size += sz
		a.notional += sz * MustFloat(f.FillPx)
		a.fee += MustFloat(f.Fee)
		a.pnl += MustFloat(f.FillPnl)
		if ts < a.firstMs {
			a.firstMs = ts
		}
		if ts > a.lastMs {
			a.lastMs = ts
		}
	}

	orders := make([]models.Order, 0, len(ids))
	for _, id := range ids {
		a := byOrder[id]
		if a.size <= 0 {
			continue
		}
		ord, ok := parents[id]
		if !ok {
			ord = models.Order{
				InstId:  a.first.InstId,
				OrdId:   id,
				Side:    a.first.Side,
				PosSide: a.first.PosSide,
				Sz:      fmt.Sprint(a.size),
				CTime:   fmt.Sprint(a.firstMs),
			}
		}
		ord.State = "filled"
		ord.AccFillSz = fmt.Sprint(a.size)
		ord.AvgPx = fmt.Sprint(a.notional / a.size)
		ord.Fee = fmt.Sprint(a.fee)
		ord.Pnl = fmt.Sprint(a.pnl)
		ord.FillTime = fmt.Sprint(a.lastMs)
		ord.UTime = fmt.Sprint(a.lastMs)
		orders = append(orders, OrderInBaseUnits(ord, instrument))
	}

	sort.SliceStable(orders, func(i, j int) bool {
		return MustInt64(orders[i].UTime) > MustInt64(orders[j].UTime)
	})
	return orders
}

func (s PositionScope) covers(ts int64, side string) bool {
	openSide := ""
	switch strings.ToLower(s.PosSide) {
	case "long":
		openSide = "buy"
	case "short":
		openSide = "sell"
	}

	if ts < s.FromMs {
		return openSide != "" && side == openSide && ts >= s.FromMs-boundarySlackMs
	}
	if s.ToMs > 0 && ts > s.ToMs {
		return openSide != "" && side != openSide && ts <= s.ToMs+boundarySlackMs
	}
	return true
}
