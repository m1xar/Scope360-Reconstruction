package helpers

import (
	"strings"

	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

type Symbols struct {
	Names    map[int64]string
	LotSizes map[int64]int64
}

func NewSymbols(names map[int64]string, full []*pb.ProtoOASymbol) Symbols {
	s := Symbols{Names: names, LotSizes: make(map[int64]int64, len(full))}
	for _, symbol := range full {
		if symbol.GetLotSize() > 0 {
			s.LotSizes[symbol.GetSymbolId()] = symbol.GetLotSize()
		}
	}
	return s
}

func (s Symbols) Name(symbolID int64) string {
	return strings.TrimSpace(s.Names[symbolID])
}

func (s Symbols) Pair(symbolID int64) string {
	return fx.NormalizePair(s.Names[symbolID])
}

func (s Symbols) Lots(symbolID int64, volumeCents int64) float64 {
	if lot := s.LotSizes[symbolID]; lot > 0 {
		return fx.Round8(float64(volumeCents) / float64(lot))
	}
	return float64(volumeCents) / 100
}

func (s Symbols) IDByName(name string) (int64, bool) {
	for id, n := range s.Names {
		if n == name {
			return id, true
		}
	}
	key := fx.NormalizePair(name)
	for id, n := range s.Names {
		if fx.NormalizePair(n) == key {
			return id, true
		}
	}
	return 0, false
}

func SymbolIDByPair(symbols []*pb.ProtoOALightSymbol, pair string) (int64, bool) {
	normalized := fx.NormalizePair(pair)
	for _, symbol := range symbols {
		if fx.NormalizePair(symbol.GetSymbolName()) == normalized {
			return symbol.GetSymbolId(), true
		}
	}
	return 0, false
}

func SymbolNameByPair(symbols []*pb.ProtoOALightSymbol, pair string) (string, bool) {
	normalized := fx.NormalizePair(pair)
	for _, symbol := range symbols {
		name := strings.TrimSpace(symbol.GetSymbolName())
		if fx.NormalizePair(name) == normalized {
			return name, true
		}
	}
	return "", false
}
