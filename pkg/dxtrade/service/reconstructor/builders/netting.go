package builders

import (
	"maps"
	"math"
	"slices"
	"strconv"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

type segment struct {
	complete bool
	side     string
	opens    []helpers.Fill
	closes   []helpers.Fill
	cash     helpers.CashSums
	orderIDs []string
	seen     map[string]bool
}

func newSegment(complete bool, signed float64) *segment {
	side := fx.Long
	if signed < 0 {
		side = fx.Short
	}
	return &segment{complete: complete, side: side, seen: map[string]bool{}}
}

func (s *segment) add(f helpers.Fill, closing bool, cash helpers.CashSums) {
	if closing {
		s.closes = append(s.closes, f)
	} else {
		s.opens = append(s.opens, f)
	}
	s.cash.Merge(cash)
	if !s.seen[f.OrderID] {
		s.seen[f.OrderID] = true
		s.orderIDs = append(s.orderIDs, f.OrderID)
	}
}

func (s *segment) position(orders map[string]models.Order, instruments map[string]models.InstrumentDetails, lotSizes map[string]float64) domain.FXPosition {
	id := s.opens[0].OrderID
	list := make([]models.Order, 0, len(s.orderIDs))
	for _, oid := range s.orderIDs {
		list = append(list, orders[oid])
	}
	return buildPosition(id, s.side, s.opens, s.closes, s.cash, BuildOrders(list, id, lotSizes), nil, nil, instruments, lotSizes, true, closeReason(list, s.closes[len(s.closes)-1]))
}

func buildNetting(orders []models.Order, open []models.Position, instruments map[string]models.InstrumentDetails, lotSizes map[string]float64) []domain.FXPosition {
	current := make(map[string]float64)
	for _, p := range open {
		q := math.Abs(p.Quantity.Float())
		if fx.PositionSide(p.Side) == fx.Short {
			q = -q
		}
		current[p.Symbol] += q
	}
	bySymbol := make(map[string][]helpers.Fill)
	fillCash := make(map[string]helpers.CashSums)
	byID := make(map[string]models.Order)
	for _, o := range orders {
		fills := helpers.Fills(o)
		if len(fills) == 0 {
			continue
		}
		byID[o.OrderID.String()] = o
		cash := distributeCash(o, fills)
		for i := range fills {
			key := fills[i].OrderID + "/" + fills[i].ExecutionCode
			if fills[i].ExecutionCode == "" {
				key = fills[i].OrderID + "/#" + strconv.Itoa(i)
			}
			fills[i].ExecutionCode = key
			fillCash[key] = cash[i]
		}
		bySymbol[o.Symbol()] = append(bySymbol[o.Symbol()], fills...)
	}

	var out []domain.FXPosition
	for _, symbol := range slices.Sorted(maps.Keys(bySymbol)) {
		fills := bySymbol[symbol]
		helpers.SortFills(fills)
		net := current[symbol]
		for _, f := range fills {
			net -= f.Signed()
		}
		if math.Abs(net) < helpers.Epsilon {
			net = 0
		}
		var seg *segment
		if net != 0 {
			seg = newSegment(false, net)
		}
		for _, f := range fills {
			signed := f.Signed()
			cash := fillCash[f.ExecutionCode]
			if net == 0 || (net > 0) == (signed > 0) {
				if net == 0 {
					seg = newSegment(true, signed)
				}
				seg.add(f, false, cash)
				net += signed
				continue
			}
			closeQty := math.Min(f.Quantity, math.Abs(net))
			ratio := closeQty / f.Quantity
			part := f
			part.Quantity = closeQty
			closing := cash.Scale(ratio)
			closing.Settlement = cash.Settlement
			seg.add(part, true, closing)
			net += math.Copysign(closeQty, signed)
			if math.Abs(net) < helpers.Epsilon*max(1, f.Quantity) {
				net = 0
				if seg.complete {
					out = append(out, seg.position(byID, instruments, lotSizes))
				}
				seg = nil
			}
			if rest := f.Quantity - closeQty; rest > helpers.Epsilon {
				opening := cash.Scale(1 - ratio)
				opening.Settlement = 0
				part.Quantity = rest
				seg = newSegment(true, signed)
				seg.add(part, false, opening)
				net += math.Copysign(rest, signed)
			}
		}
	}
	return out
}

func distributeCash(o models.Order, fills []helpers.Fill) []helpers.CashSums {
	out := make([]helpers.CashSums, len(fills))
	index := make(map[string]int, len(fills))
	for i, f := range fills {
		if f.ExecutionCode != "" {
			index[f.ExecutionCode] = i
		}
	}
	total := helpers.Quantity(fills)
	for _, tx := range o.CashTransactions {
		if i, ok := index[tx.TradeCode.String()]; ok && tx.TradeCode != "" {
			out[i].Add(tx, 1)
			continue
		}
		for i, f := range fills {
			if total > 0 {
				out[i].Add(tx, f.Quantity/total)
			}
		}
	}
	return out
}
