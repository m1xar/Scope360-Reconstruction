package builders

import (
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/executors"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
)

func BuildOpenPositionsFromFills(
	client *resty.Client,
	endpoint string,
	fills []models.RawFill,
) []domain.OpenPosition {
	if len(fills) == 0 {
		return nil
	}

	_, matched := helpers.MatchFillGroups(fills)

	type agg struct {
		coin         string
		pair         string
		side         string
		openTimeMs   int64
		openSize     float64
		closeSize    float64
		openNotional float64
	}

	aggs := make(map[string]*agg)

	for _, f := range fills {
		if _, ok := matched[f.Tid]; ok {
			continue
		}
		if !strings.Contains(f.Dir, "Open") && !strings.Contains(f.Dir, "Close") {
			continue
		}

		pair := f.Coin + f.FeeToken
		a := aggs[pair]
		if a == nil {
			a = &agg{pair: pair, coin: f.Coin}
			aggs[pair] = a
		}

		px := helpers.MustFloat(f.Px)

		sz := helpers.MustFloat(f.Sz)
		if strings.Contains(f.Dir, "Open") {
			a.openSize += sz
			a.openNotional += sz * px
			if a.side == "" {
				a.side = helpers.PositionSideFromDir(f.Dir)
			}
			if a.openTimeMs == 0 || f.Time < a.openTimeMs {
				a.openTimeMs = f.Time
			}
		} else if strings.Contains(f.Dir, "Close") {
			a.closeSize += sz
		}
	}

	out := make([]domain.OpenPosition, 0, len(aggs))
	coinPrice := make(map[string]float64, len(aggs))
	nowMs := time.Now().UnixMilli()
	const candleInterval = "1m"
	const candleLookbackMs = int64(60 * 60 * 1000)
	for _, a := range aggs {
		net := a.openSize - a.closeSize
		if net <= 0 {
			continue
		}
		entry := 0.0
		if a.openSize > 0 {
			entry = a.openNotional / a.openSize
		}

		if _, ok := coinPrice[a.coin]; !ok {
			startTime := nowMs - candleLookbackMs
			candles, err := executors.FetchAllCandlesHyperliquid(
				client,
				endpoint,
				a.coin,
				candleInterval,
				startTime,
				nowMs,
			)
			if err == nil && len(candles) > 0 {
				last := candles[len(candles)-1]
				coinPrice[a.coin] = helpers.MustFloat(last.C)
			} else {
				coinPrice[a.coin] = 0
			}
		}

		out = append(out, domain.OpenPosition{
			Pair:         a.pair,
			Amount:       helpers.Round8(net),
			Side:         a.side,
			EntryPrice:   helpers.Round8(entry),
			CurrentPrice: helpers.Round8(coinPrice[a.coin]),
			OpenTime:     time.UnixMilli(a.openTimeMs).UTC(),
		})
	}

	return out
}
