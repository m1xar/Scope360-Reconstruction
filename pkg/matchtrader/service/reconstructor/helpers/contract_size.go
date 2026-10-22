package helpers

import (
	"math"
	"sort"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
)

func (in Instruments) ContractSize(symbol string) *float64 {
	inst, ok := in[symbol]
	if !ok {
		return nil
	}
	v := inst.ContractSize.Float()
	if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}

func AttachContractSizes(positions []domain.FXPosition, in Instruments) {
	for i := range positions {
		positions[i].ContractSize = in.ContractSize(positions[i].Symbol)
	}
}

func AttachOpenContractSizes(positions []domain.FXOpenPosition, in Instruments) {
	for i := range positions {
		positions[i].ContractSize = in.ContractSize(positions[i].Symbol)
	}
}

func SymbolSpecs(in Instruments, open []models.OpenPosition, closed []models.ClosedPosition) []domain.FXSymbolSpec {
	aliases := make(map[string]string, len(in))
	for symbol, inst := range in {
		aliases[symbol] = inst.Alias
	}
	for _, p := range open {
		if _, ok := aliases[p.Symbol]; !ok && p.Symbol != "" {
			aliases[p.Symbol] = p.Alias
		}
	}
	for _, p := range closed {
		if _, ok := aliases[p.Symbol]; !ok && p.Symbol != "" {
			aliases[p.Symbol] = p.Alias
		}
	}
	out := make([]domain.FXSymbolSpec, 0, len(aliases))
	for symbol, alias := range aliases {
		out = append(out, domain.FXSymbolSpec{
			Symbol:       symbol,
			Pair:         Pair(alias, symbol),
			ContractSize: in.ContractSize(symbol),
			Source:       domain.ContractSizeMatchTrader,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}
