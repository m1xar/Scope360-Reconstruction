package executors

import (
	"fmt"
	"sort"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/connector/binance"
	"github.com/m1xar/scope360-reconstruction/pkg/binance/connector/binance/models"
)

const userTradesPath = "/fapi/v1/userTrades"

const tradesPageLimit = 1000

// TradesWindowMax is the widest startTime/endTime span userTrades accepts.
const TradesWindowMax = 7 * 24 * time.Hour

// FetchUserTradesWindow returns the trades of symbol with startMs <= time <=
// endMs (a span of at most TradesWindowMax), oldest first. fromId cannot be
// combined with a time range, so pages advance by startTime.
func FetchUserTradesWindow(client *resty.Client, symbol string, startMs, endMs int64) ([]models.Trade, error) {
	var result []models.Trade
	seen := make(map[int64]struct{})
	cursor := startMs

	for cursor <= endMs {
		params := map[string]string{
			"symbol":    symbol,
			"startTime": fmt.Sprintf("%d", cursor),
			"endTime":   fmt.Sprintf("%d", endMs),
			"limit":     fmt.Sprintf("%d", tradesPageLimit),
		}

		page, err := doWithRateLimit(func() ([]models.Trade, error) {
			return binance.DoGet[[]models.Trade](client, userTradesPath, params, 5)
		})
		if err != nil {
			if len(result) > 0 && isHTTP5xx(err) {
				break
			}
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		added := 0
		maxTs := int64(0)
		for _, fill := range page {
			if fill.Time > maxTs {
				maxTs = fill.Time
			}
			if _, ok := seen[fill.ID]; ok {
				continue
			}
			seen[fill.ID] = struct{}{}
			result = append(result, fill)
			added++
		}

		if len(page) < tradesPageLimit || added == 0 || maxTs <= cursor {
			break
		}
		cursor = maxTs
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Time == result[j].Time {
			return result[i].ID < result[j].ID
		}
		return result[i].Time < result[j].Time
	})
	return result, nil
}

// FirstUserTrade returns the symbol's oldest trade (one request from id 0),
// or nil when Binance has no trade history for it. Its time is the floor
// for a backwards walk: nothing older can exist.
func FirstUserTrade(client *resty.Client, symbol string) (*models.Trade, error) {
	page, err := doWithRateLimit(func() ([]models.Trade, error) {
		return binance.DoGet[[]models.Trade](client, userTradesPath, map[string]string{
			"symbol": symbol, "fromId": "0", "limit": "1",
		}, 5)
	})
	if err != nil {
		return nil, err
	}
	if len(page) == 0 {
		return nil, nil
	}
	return &page[0], nil
}

// FetchAllUserTrades walks a symbol's entire trade history forward by id
// (one request per 1000 trades). Used for a full-history sync, where the
// windowed backwards walk would cost a request per week of retention.
func FetchAllUserTrades(client *resty.Client, symbol string) ([]models.Trade, error) {
	var result []models.Trade
	seen := make(map[int64]struct{})
	fromID := int64(0)

	for {
		page, err := doWithRateLimit(func() ([]models.Trade, error) {
			return binance.DoGet[[]models.Trade](client, userTradesPath, map[string]string{
				"symbol": symbol,
				"fromId": fmt.Sprintf("%d", fromID),
				"limit":  fmt.Sprintf("%d", tradesPageLimit),
			}, 5)
		})
		if err != nil {
			if len(result) > 0 && isHTTP5xx(err) {
				break
			}
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		added := 0
		maxID := int64(0)
		for _, fill := range page {
			if fill.ID > maxID {
				maxID = fill.ID
			}
			if _, ok := seen[fill.ID]; ok {
				continue
			}
			seen[fill.ID] = struct{}{}
			result = append(result, fill)
			added++
		}
		if len(page) < tradesPageLimit || added == 0 || maxID < fromID {
			break
		}
		fromID = maxID + 1
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Time == result[j].Time {
			return result[i].ID < result[j].ID
		}
		return result[i].Time < result[j].Time
	})
	return result, nil
}
