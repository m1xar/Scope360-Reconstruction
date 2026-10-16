package helpers

import (
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/symbols"
)

type Instruments map[string]models.Instrument

func IndexInstruments(list []models.Instrument) Instruments {
	out := make(Instruments, len(list))
	for _, inst := range list {
		if inst.Symbol != "" {
			out[inst.Symbol] = inst
		}
	}
	return out
}

func Pair(alias, symbol string) string {
	if strings.TrimSpace(alias) != "" {
		return fx.NormalizePair(alias)
	}
	return fx.NormalizePair(symbol)
}

func (in Instruments) SymbolByPair(pair string) (string, bool) {
	if _, ok := in[pair]; ok {
		return pair, true
	}
	key := symbols.Key(fx.NormalizePair(pair))
	if key == "" {
		return "", false
	}
	best := ""
	for symbol, inst := range in {
		if symbols.Key(Pair(inst.Alias, symbol)) == key || symbols.Key(fx.NormalizePair(symbol)) == key {
			if best == "" || len(symbol) < len(best) || (len(symbol) == len(best) && symbol < best) {
				best = symbol
			}
		}
	}
	return best, best != ""
}
