package symbols

import (
	_ "embed"
	"sort"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
	registry "github.com/m1xar/scope360-reconstruction/pkg/reconstruction/symbols"
)

//go:embed seed.json
var seed []byte

var reg = registry.New(seed)

func Normalize(symbol string) string {
	return helpers.NormalizePair(symbol)
}

func Key(s string) string {
	return registry.Key(s)
}

func Universe(client *resty.Client) ([]registry.Entry, error) {
	contracts, err := executors.FetchAllContractDetails(client)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(contracts, func(i, j int) bool {
		if contracts[i].State != contracts[j].State {
			return contracts[i].State < contracts[j].State
		}
		return contracts[i].Symbol < contracts[j].Symbol
	})
	out := make([]registry.Entry, 0, len(contracts))
	for _, c := range contracts {
		out = append(out, registry.Entry{Symbol: c.Symbol, Pair: Normalize(c.Symbol)})
	}
	return out, nil
}

func Denormalize(client *resty.Client, pair string) (string, error) {
	return reg.Resolve(pair, func() ([]registry.Entry, error) {
		return Universe(client)
	})
}
