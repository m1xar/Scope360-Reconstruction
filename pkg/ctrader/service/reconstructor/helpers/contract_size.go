package helpers

import (
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func (s Symbols) ContractSize(symbolID int64) *float64 {
	lot := s.LotSizes[symbolID]
	if lot <= 0 {
		return nil
	}
	v := float64(lot) / 100
	return &v
}

func (s Symbols) SymbolSpecs() []domain.FXSymbolSpec {
	out := make([]domain.FXSymbolSpec, 0, len(s.Names))
	for id := range s.Names {
		name := s.Name(id)
		if name == "" {
			continue
		}
		out = append(out, domain.FXSymbolSpec{
			Symbol:       name,
			Pair:         s.Pair(id),
			ContractSize: s.ContractSize(id),
			Source:       domain.ContractSizeCTrader,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}
