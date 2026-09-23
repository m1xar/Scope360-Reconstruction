package symbols

import (
	_ "embed"
	"sort"

	"github.com/go-resty/resty/v2"
	blofinclient "github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/helpers"
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
	instruments, err := executors.FetchInstruments(client, blofinclient.BaseURL)
	if err != nil {
		return nil, err
	}
	rows := make([]models.Instrument, 0, len(instruments))
	for _, inst := range instruments {
		rows = append(rows, inst)
	}
	sort.Slice(rows, func(i, j int) bool {
		li, lj := rows[i].State == "live", rows[j].State == "live"
		if li != lj {
			return li
		}
		return rows[i].InstID < rows[j].InstID
	})
	out := make([]registry.Entry, 0, len(rows))
	for _, inst := range rows {
		out = append(out, registry.Entry{Symbol: inst.InstID, Pair: Normalize(inst.InstID)})
	}
	return out, nil
}

func Denormalize(client *resty.Client, pair string) (string, error) {
	return reg.Resolve(pair, func() ([]registry.Entry, error) {
		return Universe(client)
	})
}
