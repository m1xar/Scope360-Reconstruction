package executors

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
)

const candleCount = 1000

var candleTypes = map[string]struct {
	name string
	step time.Duration
}{
	"1m": {"m", time.Minute}, "5m": {"5m", 5 * time.Minute}, "15m": {"15m", 15 * time.Minute}, "30m": {"30m", 30 * time.Minute},
	"1h": {"h", time.Hour}, "2h": {"2h", 2 * time.Hour}, "4h": {"4h", 4 * time.Hour},
	"1d": {"d", 24 * time.Hour}, "1w": {"w", 7 * 24 * time.Hour},
}

func FetchCandles(ctx context.Context, c *connector.Client, symbol, interval string, from, to time.Time) ([]models.MarketEvent, error) {
	candle, ok := candleTypes[strings.ToLower(strings.TrimSpace(interval))]
	if !ok {
		return nil, fmt.Errorf("dxtrade: unsupported interval %q", interval)
	}
	account, err := c.Account(ctx)
	if err != nil {
		return nil, err
	}
	span := candle.step * (candleCount - 1)
	byTime := make(map[int64]models.MarketEvent)
	for start := from; !start.After(to); start = start.Add(span + candle.step) {
		end := start.Add(span)
		if end.After(to) {
			end = to
		}
		req := models.MarketDataRequest{
			Symbols: []string{symbol},
			Account: account.Account,
			EventTypes: []models.MarketEventRequestType{{
				Type:       models.EventCandle,
				Format:     models.FormatCompact,
				CandleType: candle.name,
				FromTime:   models.FormatTime(start),
				ToTime:     models.FormatTime(end),
				Count:      candleCount,
			}},
		}
		var list models.MarketDataList
		if err := c.Do(ctx, "marketdata_candles", http.MethodPost, "/marketdata", nil, req, &list); err != nil {
			if connector.IsStatus(err, http.StatusNotFound) {
				continue
			}
			return nil, err
		}
		for _, e := range list.Events {
			if (e.Type != "" && e.Type != models.EventCandle) || (e.Symbol != "" && e.Symbol != symbol) {
				continue
			}
			if e.Time.Before(start) || e.Time.After(end) {
				continue
			}
			byTime[e.Time.UnixMilli()] = e
		}
	}
	out := make([]models.MarketEvent, 0, len(byTime))
	for _, e := range byTime {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time.Time) })
	return out, nil
}

func FetchQuotes(ctx context.Context, c *connector.Client, symbols []string) (map[string]models.Quote, error) {
	out := make(map[string]models.Quote, len(symbols))
	if len(symbols) == 0 {
		return out, nil
	}
	account, err := c.Account(ctx)
	if err != nil {
		return nil, err
	}
	req := models.MarketDataRequest{
		Symbols:    symbols,
		Account:    account.Account,
		EventTypes: []models.MarketEventRequestType{{Type: models.EventQuote, Format: models.FormatCompact}},
	}
	var list models.MarketDataList
	if err := c.Do(ctx, "marketdata_quotes", http.MethodPost, "/marketdata", nil, req, &list); err != nil {
		if connector.IsStatus(err, http.StatusNotFound) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range list.Events {
		if e.Type == "" || e.Type == models.EventQuote {
			out[e.Symbol] = models.Quote{Symbol: e.Symbol, Bid: e.Bid.Float(), Ask: e.Ask.Float(), Time: e.Time.Time}
		}
	}
	return out, nil
}
