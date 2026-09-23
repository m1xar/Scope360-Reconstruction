package symbols

import (
	_ "embed"
	"sort"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
	registry "github.com/m1xar/scope360-reconstruction/pkg/reconstruction/symbols"
)

const quote = "USDC"

//go:embed seed.json
var seed []byte

var reg = registry.New(seed)

func Normalize(coin string) string {
	return helpers.NormalizeContractName(coin + quote)
}

func Key(s string) string {
	return registry.Key(s)
}

func Universe(client *resty.Client, endpoint string) ([]registry.Entry, error) {
	meta, err := executors.FetchMeta(client, endpoint)
	if err != nil {
		return nil, err
	}
	assets := meta.Universe
	sort.SliceStable(assets, func(i, j int) bool {
		return !assets[i].IsDelisted && assets[j].IsDelisted
	})
	out := make([]registry.Entry, 0, len(assets))
	for _, a := range assets {
		if a.Name == "" {
			continue
		}
		out = append(out, registry.Entry{Symbol: a.Name, Pair: Normalize(a.Name)})
	}
	return out, nil
}

func Denormalize(client *resty.Client, endpoint, pair string) (string, error) {
	return reg.Resolve(pair, func() ([]registry.Entry, error) {
		return Universe(client, endpoint)
	})
}
