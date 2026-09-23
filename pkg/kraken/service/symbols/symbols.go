package symbols

import (
	_ "embed"
	"sort"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/helpers"
	registry "github.com/m1xar/scope360-reconstruction/pkg/reconstruction/symbols"
)

//go:embed seed.json
var seed []byte

var reg = registry.New(seed)

func Normalize(symbol string) string {
	return helpers.NormalizePairFallback(symbol)
}

func Key(s string) string {
	return registry.Key(s)
}

func Universe(client *resty.Client) ([]registry.Entry, error) {
	tickers, err := executors.FetchTickers(client)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(tickers, func(i, j int) bool {
		ri, rj := rank(tickers[i].Symbol), rank(tickers[j].Symbol)
		if ri != rj {
			return ri < rj
		}
		return tickers[i].Symbol < tickers[j].Symbol
	})
	out := make([]registry.Entry, 0, len(tickers))
	for _, t := range tickers {
		symbol := strings.ToUpper(strings.TrimSpace(t.Symbol))
		if symbol == "" {
			continue
		}
		pair := helpers.NormalizePairText(t.Pair)
		if pair == "" {
			pair = Normalize(symbol)
		}
		out = append(out, registry.Entry{Symbol: symbol, Pair: pair})
	}
	return out, nil
}

func Denormalize(client *resty.Client, pair string) (string, error) {
	return reg.Resolve(pair, func() ([]registry.Entry, error) {
		return Universe(client)
	})
}

func rank(symbol string) int {
	s := strings.ToUpper(symbol)
	for i, prefix := range []string{"PF_", "PI_", "FI_", "FF_"} {
		if strings.HasPrefix(s, prefix) {
			return i
		}
	}
	return 4
}
