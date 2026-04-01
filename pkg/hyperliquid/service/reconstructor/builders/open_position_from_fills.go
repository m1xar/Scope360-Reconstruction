package builders

import (
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
)

func BuildOpenPositionsFromFills(
	candleRequests chan<- helpers.CandleRequest,
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
		if !helpers.IsOpen(f.Dir) && !helpers.IsClose(f.Dir) {
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
		if helpers.IsOpen(f.Dir) {
			a.openSize += sz
			a.openNotional += sz * px
			if a.side == "" {
				a.side = helpers.PositionSideFromDir(f.Dir)
			}
			if a.openTimeMs == 0 || f.Time < a.openTimeMs {
				a.openTimeMs = f.Time
			}
		} else if helpers.IsClose(f.Dir) {
			a.closeSize += sz
		}
	}

	coins := make(map[string]struct{})
	for _, a := range aggs {
		net := a.openSize - a.closeSize
		if net > 0 {
			coins[a.coin] = struct{}{}
		}
	}

	if len(coins) == 0 {
		return nil
	}

	nowMs := time.Now().UnixMilli()
	const candleLookbackMs = int64(60 * 60 * 1000)

	type priceReply struct {
		coin    string
		replyCh chan helpers.CandleResponse
	}

	replies := make([]priceReply, 0, len(coins))
	for coin := range coins {
		replyCh := make(chan helpers.CandleResponse, 1)
		candleRequests <- helpers.CandleRequest{
			Coin:     coin,
			Interval: "1m",
			StartMs:  nowMs - candleLookbackMs,
			EndMs:    nowMs,
			ReplyCh:  replyCh,
		}
		replies = append(replies, priceReply{coin: coin, replyCh: replyCh})
	}

	coinPrice := make(map[string]float64, len(coins))
	for _, r := range replies {
		resp := <-r.replyCh
		if resp.Err == nil && len(resp.Candles) > 0 {
			last := resp.Candles[len(resp.Candles)-1]
			coinPrice[r.coin] = helpers.MustFloat(last.C)
		}
	}

	out := make([]domain.OpenPosition, 0, len(aggs))
	for _, a := range aggs {
		net := a.openSize - a.closeSize
		if net <= 0 {
			continue
		}
		entry := 0.0
		if a.openSize > 0 {
			entry = a.openNotional / a.openSize
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
