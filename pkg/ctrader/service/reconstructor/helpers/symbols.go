package helpers

import (
	"strings"

	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

func SymbolName(symbols map[int64]string, symbolID int64) string {
	if name := symbols[symbolID]; name != "" {
		return strings.ReplaceAll(name, "/", "")
	}
	return ""
}

func SymbolIDByPair(symbols []*pb.ProtoOALightSymbol, pair string) (int64, bool) {
	normalized := strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(pair)), "/", "")
	for _, symbol := range symbols {
		name := strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(symbol.GetSymbolName())), "/", "")
		if name == normalized {
			return symbol.GetSymbolId(), true
		}
	}
	return 0, false
}

func NormalizePair(name string) string {
	return strings.ReplaceAll(strings.TrimSpace(name), "/", "")
}

func SymbolNameByPair(symbols []*pb.ProtoOALightSymbol, pair string) (string, bool) {
	normalized := strings.ToUpper(NormalizePair(pair))
	for _, symbol := range symbols {
		name := strings.TrimSpace(symbol.GetSymbolName())
		if strings.ToUpper(NormalizePair(name)) == normalized {
			return name, true
		}
	}
	return "", false
}
