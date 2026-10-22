package helpers

import (
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

// ContractSize returns the symbol's contract size in base-asset units per lot.
// cTrader reports ProtoOASymbol.lotSize in cents of units (like every volume
// in Open API: "1000 in protocol means 10.00 units"), so the size is
// lotSize/100, e.g. EURUSD lotSize 10000000 -> 100000. nil when the symbol
// has no lotSize; Lots() then falls back to volume/100.
func (s Symbols) ContractSize(symbolID int64) *float64 {
	lot := s.LotSizes[symbolID]
	if lot <= 0 {
		return nil
	}
	v := float64(lot) / 100
	return &v
}

// SymbolSpecs lists every symbol of the light list, sorted by name; symbols
// without a loaded lotSize get ContractSize nil.
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
