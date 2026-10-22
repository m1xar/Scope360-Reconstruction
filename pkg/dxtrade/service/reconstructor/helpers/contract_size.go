package helpers

import (
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

func ContractSize(lotSizes map[string]float64, symbol string) *float64 {
	size, ok := lotSizes[symbol]
	if !ok || size <= 0 {
		return nil
	}
	return &size
}

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
