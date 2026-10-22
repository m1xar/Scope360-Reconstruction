package reconstructor

import (
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/executors"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/workers"
)

type usdPricer struct {
	client  *resty.Client
	baseURL string
	mu      sync.Mutex
	cache   map[string]float64
}

func newUSDPricer(client *resty.Client, baseURL string) *usdPricer {
	return &usdPricer{client: client, baseURL: baseURL, cache: map[string]float64{}}
}

func (p *usdPricer) price(ccy string, ts int64) (float64, bool) {
	if helpers.IsStableAsset(ccy) {
		return 1, true
	}
	minute := ts - ts%60000
	key := ccy + "@" + strconv.FormatInt(minute, 10)

	p.mu.Lock()
	defer p.mu.Unlock()
	if v, ok := p.cache[key]; ok {
		return v, v > 0
	}

	px := 0.0
	for _, m := range usdMarkets(ccy) {
		if close := closeAt(p.client, p.baseURL, m.instId, minute); close > 0 {
			px = close
			if m.inverse {
				px = 1 / close
			}
			break
		}
	}
	p.cache[key] = px
	return px, px > 0
}

type usdMarket struct {
	instId  string
	inverse bool
}

func usdMarkets(ccy string) []usdMarket {
	return []usdMarket{
		{instId: ccy + "-USDT"},
		{instId: ccy + "-USDC"},
		{instId: "USDT-" + ccy, inverse: true},
		{instId: "USDC-" + ccy, inverse: true},
	}
}

func closeAt(client *resty.Client, baseURL, instId string, minute int64) float64 {
	candles, err := executors.FetchCandles(client, baseURL, instId, "1m", minute, minute+60000)
	if err != nil || len(candles) == 0 {
		return 0
	}
	for _, c := range candles {
		if helpers.MustInt64(c.Ts) == minute {
			return helpers.MustFloat(c.C)
		}
	}
	return helpers.MustFloat(candles[0].C)
}

func spotLegs(orders []models.Order, pricer *usdPricer) []helpers.SpotLeg {
	var legs []helpers.SpotLeg
	for _, ord := range orders {
		if !helpers.IsFilled(ord) {
			continue
		}
		parts := strings.SplitN(ord.InstId, "-", 2)
		if len(parts) != 2 {
			continue
		}
		base, quote := parts[0], parts[1]
		qty := helpers.MustFloat(ord.AccFillSz)
		px := helpers.MustFloat(ord.AvgPx)
		ts := helpers.MustInt64(ord.UTime)
		if t := helpers.MustInt64(ord.FillTime); t > 0 {
			ts = t
		}

		quoteUSD, ok := pricer.price(quote, ts)
		if !ok {
			continue
		}
		fee := math.Abs(helpers.MustFloat(ord.Fee))
		leg := helpers.SpotLeg{
			Asset:   base,
			Quote:   quote,
			InstId:  ord.InstId,
			OrdId:   ord.OrdId,
			OrdType: ord.OrdType,
			Side:    strings.ToLower(ord.Side),
			Qty:     qty,
			PxUSD:   px * quoteUSD,
			Ts:      ts,
		}
		switch feeCcy := ord.FeeCcy; {
		case fee == 0:
		case strings.EqualFold(feeCcy, base):
			leg.FeeBase = fee
		case strings.EqualFold(feeCcy, quote):
			leg.FeeUSD = fee * quoteUSD
		default:
			if feeUSD, ok := pricer.price(feeCcy, ts); ok {
				leg.FeeUSD = fee * feeUSD
			}
		}
		legs = append(legs, leg)

		if !helpers.IsStableAsset(quote) {
			other := "sell"
			if leg.Side == "sell" {
				other = "buy"
			}
			legs = append(legs, helpers.SpotLeg{
				Asset:   quote,
				Quote:   "USDT",
				InstId:  quote + "-USDT",
				OrdId:   ord.OrdId,
				OrdType: ord.OrdType,
				Side:    other,
				Qty:     qty * px,
				PxUSD:   quoteUSD,
				Ts:      ts,
			})
		}
	}
	return legs
}

func transferLegs(bills []models.Bill, pricer *usdPricer) []helpers.SpotLeg {
	var legs []helpers.SpotLeg
	for _, b := range bills {
		if b.Type != "1" || helpers.IsStableAsset(b.Ccy) {
			continue
		}
		var side string
		switch b.SubType {
		case "11":
			side = "buy"
		case "12":
			side = "sell"
		default:
			continue
		}
		qty := math.Abs(helpers.MustFloat(b.Sz))
		if qty == 0 {
			continue
		}
		ts := helpers.MustInt64(b.Ts)
		px, ok := pricer.price(b.Ccy, ts)
		if !ok {
			continue
		}
		legs = append(legs, helpers.SpotLeg{
			Asset:    b.Ccy,
			Quote:    "USDT",
			InstId:   b.Ccy + "-USDT",
			OrdId:    b.BillId,
			OrdType:  "market",
			Side:     side,
			Qty:      qty,
			PxUSD:    px,
			Ts:       ts,
			Transfer: true,
		})
	}
	return legs
}

func spotHoldings(balance models.Balance) []helpers.SpotHolding {
	out := make([]helpers.SpotHolding, 0, len(balance.Details))
	for _, d := range balance.Details {
		qty := helpers.MustFloat(d.CashBal)
		if qty <= 0 {
			continue
		}
		price := 0.0
		if eq := helpers.MustFloat(d.Eq); eq > 0 {
			price = helpers.MustFloat(d.EqUsd) / eq
		}
		out = append(out, helpers.SpotHolding{
			Asset:    d.Ccy,
			Qty:      qty,
			AvgPx:    helpers.MustFloat(d.OpenAvgPx),
			PriceUSD: price,
		})
	}
	return out
}

func excursionMarket(symbol string) string {
	parts := strings.SplitN(symbol, "-", 2)
	if len(parts) == 2 && helpers.IsStableAsset(parts[1]) {
		return symbol
	}
	return parts[0] + "-USDT"
}

func applySpotExcursions(client *resty.Client, baseURL string, positions []domain.Position) {
	requests := make(chan helpers.CandleRequest, defaultCandleWorkers)
	workers.StartCandleWorkers(client, baseURL, requests, defaultCandleWorkers)

	type pending struct {
		idx     int
		replyCh chan helpers.CandleResponse
	}
	queue := make([]pending, 0, len(positions))
	for i, pos := range positions {
		if pos.CreatedAt.IsZero() || pos.ClosedAt == nil || !pos.ClosedAt.After(pos.CreatedAt) {
			continue
		}
		replyCh := make(chan helpers.CandleResponse, 1)
		requests <- helpers.CandleRequest{
			InstId:  excursionMarket(pos.Symbol),
			Bar:     "1m",
			StartMs: pos.CreatedAt.UnixMilli(),
			EndMs:   pos.ClosedAt.UnixMilli(),
			ReplyCh: replyCh,
		}
		queue = append(queue, pending{idx: i, replyCh: replyCh})
	}
	close(requests)

	for _, p := range queue {
		resp := <-p.replyCh
		if resp.Err == nil {
			high, low := helpers.GetHighLow(resp.Candles)
			helpers.ApplyMAEMFE(&positions[p.idx], high, low)
		}
	}
}
