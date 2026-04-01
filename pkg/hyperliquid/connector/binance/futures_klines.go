package binance

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
	hlmodels "github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
)

const futuresBaseURL = "https://fapi.binance.com"

func FetchFuturesKlinesPaged(
	client *resty.Client,
	symbol string,
	interval string,
	startTimeMs int64,
	endTimeMs int64,
	limit int,
) ([]hlmodels.HyperliquidCandle, error) {
	if client == nil {
		client = resty.New()
	}

	if limit <= 0 {
		limit = 499
	}

	symbol = normalizeSymbol(symbol)

	out := make([]hlmodels.HyperliquidCandle, 0)
	nextStart := startTimeMs

	for {
		if endTimeMs > 0 && nextStart > endTimeMs {
			break
		}

		var raw [][]any
		err := fetchFuturesKlines(
			client,
			symbol,
			interval,
			nextStart,
			endTimeMs,
			limit,
			&raw,
		)
		if err != nil {
			return nil, err
		}

		if len(raw) == 0 {
			break
		}

		candles, lastOpen, err := mapKlines(raw, symbol, interval)
		if err != nil {
			return nil, err
		}

		out = append(out, candles...)

		if len(raw) < limit {
			break
		}

		if lastOpen <= 0 {
			break
		}

		nextStart = lastOpen + 1
	}

	return out, nil
}

func fetchFuturesKlines(
	client *resty.Client,
	symbol string,
	interval string,
	startTimeMs int64,
	endTimeMs int64,
	limit int,
	out *[]([]any),
) error {
	req := client.R().
		SetQueryParam("symbol", symbol).
		SetQueryParam("interval", interval).
		SetQueryParam("limit", fmt.Sprint(limit)).
		SetResult(out)

	if startTimeMs > 0 {
		req.SetQueryParam("startTime", fmt.Sprint(startTimeMs))
	}
	if endTimeMs > 0 {
		req.SetQueryParam("endTime", fmt.Sprint(endTimeMs))
	}

	resp, err := req.Get(futuresBaseURL + "/fapi/v1/klines")
	if err != nil {
		return err
	}

	if resp.IsError() {
		return errors.New(resp.Status())
	}

	return nil
}

func mapKlines(raw [][]any, symbol string, interval string) ([]hlmodels.HyperliquidCandle, int64, error) {
	out := make([]hlmodels.HyperliquidCandle, 0, len(raw))
	var lastOpen int64

	for _, row := range raw {
		if len(row) < 9 {
			return nil, 0, errors.New("binance klines response row has insufficient length")
		}

		openTime, ok := asInt64(row[0])
		if !ok {
			return nil, 0, errors.New("binance klines open time invalid")
		}
		closeTime, ok := asInt64(row[6])
		if !ok {
			return nil, 0, errors.New("binance klines cls time invalid")
		}
		numTrades, _ := asInt64(row[8])

		open, ok := row[1].(string)
		if !ok {
			return nil, 0, errors.New("binance klines open invalid")
		}
		high, ok := row[2].(string)
		if !ok {
			return nil, 0, errors.New("binance klines high invalid")
		}
		low, ok := row[3].(string)
		if !ok {
			return nil, 0, errors.New("binance klines low invalid")
		}
		cls, ok := row[4].(string)
		if !ok {
			return nil, 0, errors.New("binance klines cls invalid")
		}
		volume, ok := row[5].(string)
		if !ok {
			return nil, 0, errors.New("binance klines volume invalid")
		}

		out = append(out, hlmodels.HyperliquidCandle{
			T:  closeTime,
			C:  cls,
			H:  high,
			I:  interval,
			L:  low,
			N:  int(numTrades),
			O:  open,
			S:  symbol,
			T0: openTime,
			V:  volume,
		})

		lastOpen = openTime
	}

	return out, lastOpen, nil
}

func asInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int64:
		return t, true
	case int:
		return int64(t), true
	default:
		return 0, false
	}
}

func normalizeSymbol(symbol string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	if s == "" {
		return s
	}
	if strings.HasSuffix(s, "USDC") {
		return s
	}
	return s + "USDC"
}
