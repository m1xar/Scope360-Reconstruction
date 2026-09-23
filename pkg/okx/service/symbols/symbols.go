package symbols

import (
	_ "embed"
	"sort"
	"strconv"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
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

func Universe(client *resty.Client, baseURL string) ([]registry.Entry, error) {
	swaps, err := executors.FetchInstrumentsByType(client, baseURL, "SWAP")
	if err != nil {
		return nil, err
	}
	futures, err := executors.FetchInstrumentsByType(client, baseURL, "FUTURES")
	if err != nil {
		return nil, err
	}
	sort.SliceStable(swaps, func(i, j int) bool {
		return live(swaps[i]) && !live(swaps[j])
	})
	sort.SliceStable(futures, func(i, j int) bool {
		if live(futures[i]) != live(futures[j]) {
			return live(futures[i])
		}
		return expiry(futures[i]) < expiry(futures[j])
	})
	out := make([]registry.Entry, 0, len(swaps)+len(futures))
	for _, inst := range append(swaps, futures...) {
		out = append(out, registry.Entry{Symbol: inst.InstID, Pair: Normalize(inst.InstID)})
	}
	return out, nil
}

func Denormalize(client *resty.Client, baseURL, pair string) (string, error) {
	return reg.Resolve(pair, func() ([]registry.Entry, error) {
		return Universe(client, baseURL)
	})
}

func live(inst models.Instrument) bool {
	return inst.State == "" || inst.State == "live"
}

func expiry(inst models.Instrument) int64 {
	ms, _ := strconv.ParseInt(inst.ExpTime, 10, 64)
	return ms
}
