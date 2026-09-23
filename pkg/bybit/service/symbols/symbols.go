package symbols

import (
	_ "embed"
	"sort"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit/models"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor/helpers"
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
	instruments, err := executors.FetchInstruments(client)
	if err != nil {
		return nil, err
	}
	rows := make([]models.Instrument, 0, len(instruments))
	for _, inst := range instruments {
		rows = append(rows, inst)
	}
	sort.Slice(rows, func(i, j int) bool {
		ri, rj := rank(rows[i]), rank(rows[j])
		if ri != rj {
			return ri < rj
		}
		di, dj := rows[i].DeliveryTime.Int64(), rows[j].DeliveryTime.Int64()
		if di != dj {
			return di < dj
		}
		return rows[i].Symbol < rows[j].Symbol
	})
	out := make([]registry.Entry, 0, len(rows))
	for _, inst := range rows {
		out = append(out, registry.Entry{Symbol: inst.Symbol, Pair: Normalize(inst.Symbol)})
	}
	return out, nil
}

func Denormalize(client *resty.Client, pair string) (string, error) {
	return reg.Resolve(pair, func() ([]registry.Entry, error) {
		return Universe(client)
	})
}

func rank(inst models.Instrument) int {
	r := 0
	if inst.ContractType != "LinearPerpetual" {
		r += 2
	}
	if inst.Status != "Trading" {
		r++
	}
	return r
}
