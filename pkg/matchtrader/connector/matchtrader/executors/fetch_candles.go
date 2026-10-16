package executors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	connector "github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
)

const candlesPerRequest = 1000

var intervals = map[string]struct {
	name string
	step time.Duration
}{
	"1M": {"M1", time.Minute}, "M1": {"M1", time.Minute},
	"5M": {"M5", 5 * time.Minute}, "M5": {"M5", 5 * time.Minute},
	"15M": {"M15", 15 * time.Minute}, "M15": {"M15", 15 * time.Minute},
	"30M": {"M30", 30 * time.Minute}, "M30": {"M30", 30 * time.Minute},
	"1H": {"H1", time.Hour}, "H1": {"H1", time.Hour},
	"4H": {"H4", 4 * time.Hour}, "H4": {"H4", 4 * time.Hour},
	"1D": {"D1", 24 * time.Hour}, "D1": {"D1", 24 * time.Hour},
	"1W": {"W1", 7 * 24 * time.Hour}, "W1": {"W1", 7 * 24 * time.Hour},
	"MN1": {"MN1", 31 * 24 * time.Hour},
}

func lookupInterval(interval string) (string, time.Duration, error) {
	v, ok := intervals[strings.ToUpper(strings.TrimSpace(interval))]
	if !ok {
		return "", 0, fmt.Errorf("matchtrader: unsupported interval %q", interval)
	}
	return v.name, v.step, nil
}

func FetchCandles(ctx context.Context, c *connector.Client, symbol, interval string, from, to time.Time) ([]models.Bar, error) {
	name, step, err := lookupInterval(interval)
	if err != nil {
		return nil, err
	}
	chunk := step * candlesPerRequest
	seen := make(map[int64]struct{})
	var out []models.Bar
	for start := from; !start.After(to); start = start.Add(chunk) {
		end := start.Add(chunk - time.Millisecond)
		if end.After(to) {
			end = to
		}
		q := url.Values{}
		q.Set("symbol", symbol)
		q.Set("interval", name)
		q.Set("from", start.UTC().Format("2006-01-02T15:04:05Z"))
		q.Set("to", end.UTC().Format("2006-01-02T15:04:05Z"))
		var raw json.RawMessage
		if err := c.Do(ctx, http.MethodGet, "candles", q, nil, &raw); err != nil {
			return nil, err
		}
		bars, err := decodeList[models.Bar]("candles", raw, "candles")
		if err != nil {
			return nil, err
		}
		for _, b := range bars {
			ms := b.Time.UnixMilli()
			if _, ok := seen[ms]; ok {
				continue
			}
			seen[ms] = struct{}{}
			out = append(out, b)
		}
	}
	return out, nil
}
