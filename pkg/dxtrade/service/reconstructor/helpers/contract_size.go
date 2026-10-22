package helpers

import (
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

// ContractSize returns the instrument lotSize (base-asset units per lot) from
// the map built by LotSizes, or nil when the platform did not report one.
// Lots() falls back to raw units in that case, so nil also flags such Amounts.
func ContractSize(lotSizes map[string]float64, symbol string) *float64 {
	size, ok := lotSizes[symbol]
	if !ok || size <= 0 {
		return nil
	}
	return &size
}

// SymbolSpecs lists every instrument with a lotSize plus traded symbols
// without one (ContractSize nil), sorted by symbol.
func SymbolSpecs(lotSizes map[string]float64, positions []models.Position, orders []models.Order) []domain.FXSymbolSpec {
	seen := make(map[string]bool, len(lotSizes))
	for symbol := range lotSizes {
		seen[symbol] = true
	}
	for _, p := range positions {
		if p.Symbol != "" {
			seen[p.Symbol] = true
		}
	}
	for _, o := range orders {
		if s := o.Symbol(); s != "" {
			seen[s] = true
		}
	}
	out := make([]domain.FXSymbolSpec, 0, len(seen))
	for symbol := range seen {
		out = append(out, domain.FXSymbolSpec{
			Symbol:       symbol,
			Pair:         fx.NormalizePair(symbol),
			ContractSize: ContractSize(lotSizes, symbol),
			Source:       domain.ContractSizeDXtrade,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}
