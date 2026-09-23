package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	binanceclient "github.com/m1xar/scope360-reconstruction/pkg/binance/connector/binance"
	binancesymbols "github.com/m1xar/scope360-reconstruction/pkg/binance/service/symbols"
	blofinclient "github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin"
	blofinsymbols "github.com/m1xar/scope360-reconstruction/pkg/blofin/service/symbols"
	bybitclient "github.com/m1xar/scope360-reconstruction/pkg/bybit/connector/bybit"
	bybitsymbols "github.com/m1xar/scope360-reconstruction/pkg/bybit/service/symbols"
	hlclient "github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid"
	hlsymbols "github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/symbols"
	krakenclient "github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken"
	krakensymbols "github.com/m1xar/scope360-reconstruction/pkg/kraken/service/symbols"
	mexcclient "github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc"
	mexcsymbols "github.com/m1xar/scope360-reconstruction/pkg/mexc/service/symbols"
	okxclient "github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx"
	okxsymbols "github.com/m1xar/scope360-reconstruction/pkg/okx/service/symbols"
	orderlyclient "github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly"
	orderlysymbols "github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/service/symbols"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/symbols"
)

const hyperliquidEndpoint = "https://api.hyperliquid.xyz/info"

type source struct {
	name  string
	path  string
	fetch func() ([]symbols.Entry, error)
}

func main() {
	root := "pkg"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	sources := []source{
		{"okx", "okx/service/symbols/seed.json", func() ([]symbols.Entry, error) {
			return okxsymbols.Universe(okxclient.NewBaseClient(), okxclient.BaseURL(okxclient.RegionGlobal))
		}},
		{"binance", "binance/service/symbols/seed.json", func() ([]symbols.Entry, error) {
			return binancesymbols.Universe(binanceclient.NewBaseClient())
		}},
		{"bybit", "bybit/service/symbols/seed.json", func() ([]symbols.Entry, error) {
			return bybitsymbols.Universe(bybitclient.NewBaseClient())
		}},
		{"blofin", "blofin/service/symbols/seed.json", func() ([]symbols.Entry, error) {
			return blofinsymbols.Universe(blofinclient.NewBaseClient())
		}},
		{"kraken", "kraken/service/symbols/seed.json", func() ([]symbols.Entry, error) {
			return krakensymbols.Universe(krakenclient.NewPublicClient())
		}},
		{"mexc", "mexc/service/symbols/seed.json", func() ([]symbols.Entry, error) {
			return mexcsymbols.Universe(mexcclient.NewPublicClient())
		}},
		{"hyperliquid", "hyperliquid/service/symbols/seed.json", func() ([]symbols.Entry, error) {
			return hlsymbols.Universe(hlclient.NewBaseClient(), hyperliquidEndpoint)
		}},
		{"orderly", "orderly/perptools/service/symbols/seed.json", func() ([]symbols.Entry, error) {
			return orderlysymbols.Universe(orderlyclient.NewClient(orderlyclient.Config{}))
		}},
	}

	failed := false
	for _, src := range sources {
		entries, err := src.fetch()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%-12s error: %v\n", src.name, err)
			failed = true
			continue
		}
		if err := write(filepath.Join(root, src.path), entries); err != nil {
			fmt.Fprintf(os.Stderr, "%-12s write error: %v\n", src.name, err)
			failed = true
			continue
		}
		fmt.Printf("%-12s %5d symbols, %3d collisions%s\n", src.name, len(entries), len(collisions(entries)), collisionSample(entries))
	}
	if failed {
		os.Exit(1)
	}
}

func write(path string, entries []symbols.Entry) error {
	var b strings.Builder
	b.WriteString("[\n")
	for i, e := range entries {
		line, err := json.Marshal(e)
		if err != nil {
			return err
		}
		b.WriteString(" ")
		b.Write(line)
		if i < len(entries)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("]\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func collisions(entries []symbols.Entry) map[string][]string {
	byKey := make(map[string][]string)
	for _, e := range entries {
		key := symbols.Key(e.Pair)
		byKey[key] = append(byKey[key], e.Symbol)
	}
	out := make(map[string][]string)
	for key, syms := range byKey {
		if len(syms) > 1 {
			out[key] = syms
		}
	}
	return out
}

func collisionSample(entries []symbols.Entry) string {
	all := collisions(entries)
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 3 {
		keys = keys[:3]
	}
	s := ""
	for _, k := range keys {
		s += fmt.Sprintf("  %s=%v", k, all[k])
	}
	return s
}
