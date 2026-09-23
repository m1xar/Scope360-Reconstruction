package symbols

import (
	_ "embed"
	"sort"
	"strings"

	connector "github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/reconstructor/helpers"
	registry "github.com/m1xar/scope360-reconstruction/pkg/reconstruction/symbols"
)

//go:embed seed.json
var seed []byte

var reg = registry.New(seed)

func Normalize(symbol string) string {
	return helpers.NormalizeSymbol(symbol)
}

func Key(s string) string {
	return registry.Key(s)
}

func Universe(client *connector.Client) ([]registry.Entry, error) {
	rows, err := executors.FetchSymbolsInfo(client)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(rows, func(i, j int) bool {
		pi, pj := strings.HasPrefix(rows[i].Symbol, "PERP_"), strings.HasPrefix(rows[j].Symbol, "PERP_")
		if pi != pj {
			return pi
		}
		return rows[i].Symbol < rows[j].Symbol
	})
	out := make([]registry.Entry, 0, len(rows))
	for _, r := range rows {
		if r.Symbol == "" {
			continue
		}
		out = append(out, registry.Entry{Symbol: r.Symbol, Pair: Normalize(r.Symbol)})
	}
	return out, nil
}

func Denormalize(client *connector.Client, pair string) (string, error) {
	return reg.Resolve(pair, func() ([]registry.Entry, error) {
		return Universe(client)
	})
}
