package helpers

import (
	"math"
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

type CashEntry struct {
	models.CashTransaction
	PositionCode string
	OrderID      string
}

func Ledger(orders []models.Order, transfers []models.CashTransfer) []CashEntry {
	seen := make(map[string]bool)
	var out []CashEntry
	add := func(tx models.CashTransaction, positionCode, orderID string) {
		key := tx.Key()
		if seen[key] {
			return
		}
		seen[key] = true
		if code := tx.PositionCode.String(); code != "" {
			positionCode = code
		}
		if id := tx.OrderID.String(); id != "" && orderID == "" {
			orderID = id
		}
		out = append(out, CashEntry{CashTransaction: tx, PositionCode: positionCode, OrderID: orderID})
	}
	for _, o := range orders {
		for _, tx := range o.CashTransactions {
			add(tx, o.PositionCode(), o.OrderID.String())
		}
	}
	for _, t := range transfers {
		for _, tx := range t.CashTransactions {
			if tx.TransactionTime.IsZero() {
				tx.TransactionTime = t.TransactionTime
			}
			add(tx, "", "")
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TransactionTime.Before(out[j].TransactionTime.Time) })
	return out
}

func ByPosition(ledger []CashEntry) map[string][]CashEntry {
	out := make(map[string][]CashEntry)
	for _, e := range ledger {
		if e.PositionCode != "" {
			out[e.PositionCode] = append(out[e.PositionCode], e)
		}
	}
	return out
}

func BalanceEvents(ledger []CashEntry) []fx.BalanceEvent {
	events := make([]fx.BalanceEvent, 0, len(ledger))
	for _, e := range ledger {
		if !e.TransactionTime.IsZero() {
			events = append(events, fx.BalanceEvent{Time: e.TransactionTime.Time, Delta: e.Value.Float()})
		}
	}
	return events
}

type CashSums struct {
	Settlement float64
	Commission float64
	Financing  float64
	Other      float64
}

func (s *CashSums) Add(tx models.CashTransaction, share float64) {
	v := tx.Value.Float() * share
	switch tx.Type {
	case models.CashSettlement:
		s.Settlement += v
	case models.CashCommission:
		s.Commission += v
	case models.CashFinancing:
		s.Financing += v
	case models.CashRebate, models.CashCost, models.CashDividend:
		s.Other += v
	}
}

func (s *CashSums) Merge(o CashSums) {
	s.Settlement += o.Settlement
	s.Commission += o.Commission
	s.Financing += o.Financing
	s.Other += o.Other
}

func (s CashSums) Scale(k float64) CashSums {
	return CashSums{Settlement: s.Settlement * k, Commission: s.Commission * k, Financing: s.Financing * k, Other: s.Other * k}
}

func (s CashSums) Net() float64 {
	return s.Settlement + s.Financing - math.Abs(s.Commission) + s.Other
}

func SumEntries(entries []CashEntry) CashSums {
	var s CashSums
	for _, e := range entries {
		s.Add(e.CashTransaction, 1)
	}
	return s
}

func SumTransactions(txs []models.CashTransaction) CashSums {
	var s CashSums
	for _, tx := range txs {
		s.Add(tx, 1)
	}
	return s
}
