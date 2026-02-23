package builders

import (
	"strings"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
)

func BuildOpenPositionsFromFills(fills []models.RawFill) []domain.OpenPosition {
	if len(fills) == 0 {
		return nil
	}

	_, matched := helpers.MatchFillGroups(fills)

	type agg struct {
		pair         string
		side         string
		openSize     float64
		closeSize    float64
		openNotional float64
		lastPrice    float64
		size         float64
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
			a = &agg{pair: pair}
			aggs[pair] = a
		}

		px := helpers.MustFloat(f.Px)
		a.lastPrice = px

		sz := helpers.MustFloat(f.Sz)
		if strings.Contains(f.Dir, "Open") {
			a.openSize += sz
			a.openNotional += sz * px
			if a.side == "" {
				a.side = helpers.SideFromDir(f.Dir)
			}
		} else if strings.Contains(f.Dir, "Close") {
			a.closeSize += sz
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
			EntryPrice:   helpers.Round8(entry),
			CurrentPrice: helpers.Round8(a.lastPrice),
		})
	}

	return out
}
