package executors

import "math"

const candlesMaxLimit = 1000

const klineHistoryPath = "/v1/tv/kline_history"

// supportedIntervals maps standard (Hyperliquid) interval format to Orderly kline history resolution format.
var supportedIntervals = map[string]string{
	"1m": "1m", "5m": "5m", "15m": "15m", "30m": "30m",
	"1h": "1h", "4h": "4h", "12h": "12h",
	"1d": "1d", "1w": "1w", "1M": "1mon", "1y": "1y",
}

type klineHistoryResponse struct {
	S string    `json:"s"`
	O []float64 `json:"o"`
	C []float64 `json:"c"`
	H []float64 `json:"h"`
	L []float64 `json:"l"`
	V []float64 `json:"v"`
	A []float64 `json:"a"`
	T []int64   `json:"t"` // Unix seconds
}

func minLen(first int, rest ...int) int {
	min := first
	for _, v := range rest {
		if v < min {
			min = v
		}
	}
	return min
}

func intervalDurationMs(interval string) (int64, bool) {
	switch interval {
	case "1m":
		return 60_000, true
	case "5m":
		return 5 * 60_000, true
	case "15m":
		return 15 * 60_000, true
	case "30m":
		return 30 * 60_000, true
	case "1h":
		return 60 * 60_000, true
	case "4h":
		return 4 * 60 * 60_000, true
	case "12h":
		return 12 * 60 * 60_000, true
	case "1d":
		return 24 * 60 * 60_000, true
	case "1w":
		return 7 * 24 * 60 * 60_000, true
	case "1M":
		return 30 * 24 * 60 * 60_000, true
	case "1y":
		return 365 * 24 * 60 * 60_000, true
	default:
		return 0, false
	}
}

var maxInt64 = int64(math.MaxInt64)
