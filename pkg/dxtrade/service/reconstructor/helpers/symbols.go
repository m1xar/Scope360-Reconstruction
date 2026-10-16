package helpers

import (
	"math"

	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
	registry "github.com/m1xar/scope360-reconstruction/pkg/reconstruction/symbols"
)

func LotSizes(instruments []models.Instrument) map[string]float64 {
	out := make(map[string]float64, len(instruments))
	for _, in := range instruments {
		if in.LotSize > 0 {
			out[in.Symbol] = in.LotSize.Float()
		}
	}
	return out
}

func Lots(lotSizes map[string]float64, symbol string, units float64) float64 {
	if size := lotSizes[symbol]; size > 0 {
		return fx.Round8(units / size)
	}
	return units
}

func InstrumentMap(instruments []models.InstrumentDetails) map[string]models.InstrumentDetails {
	out := make(map[string]models.InstrumentDetails, len(instruments))
	for _, in := range instruments {
		out[in.Symbol] = in
	}
	return out
}

func SymbolByPair(instruments []models.InstrumentDetails, pair string) (string, bool) {
	key := registry.Key(fx.NormalizePair(pair))
	for _, in := range instruments {
		if in.Symbol == pair {
			return in.Symbol, true
		}
	}
	for _, in := range instruments {
		if registry.Key(fx.NormalizePair(in.Symbol)) == key {
			return in.Symbol, true
		}
	}
	return "", false
}

func Multiplier(in models.InstrumentDetails) uint32 {
	if len(in.MarginRate.Tiers) == 0 {
		return 0
	}
	rate := in.MarginRate.Tiers[0].Value.Float()
	if rate <= 0 || math.IsInf(rate, 0) || math.IsNaN(rate) {
		return 0
	}
	return uint32(math.Round(1 / rate))
}

func AccountLeverage(instruments []models.InstrumentDetails) uint64 {
	var best uint32
	for _, in := range instruments {
		if in.Type != "FOREX" && in.Type != "CFD_FOREX" && in.Type != "CURRENCY" {
			continue
		}
		if m := Multiplier(in); m > best {
			best = m
		}
	}
	return uint64(best)
}
