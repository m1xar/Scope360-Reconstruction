package builders

import (
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/executors"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
	"math"
	"net/http"
)

func BuildOpenPositionsFromClearinghouse(
	state models.ClearinghouseState,
	client *http.Client,
	endpoint string,
) ([]domain.OpenPosition, error) {
	out := make([]domain.OpenPosition, 0, len(state.AssetPositions))
	coinPrice := make(map[string]float64)

	for _, ap := range state.AssetPositions {
		pos := ap.Position
		if _, ok := coinPrice[pos.Coin]; !ok {
			book, err := executors.FetchL2Book(client, endpoint, pos.Coin)
			if err != nil {
				return nil, err
			}
			if len(book.Levels) == 0 || len(book.Levels[0]) == 0 {
				coinPrice[pos.Coin] = 0
			} else {
				coinPrice[pos.Coin] = helpers.MustFloat(book.Levels[0][0].Px)
			}
		}

		size := helpers.MustFloat(pos.Szi)
		entry := helpers.MustFloat(pos.EntryPx)
		pnl := helpers.MustFloat(pos.UnrealizedPnl)
		liq := helpers.MustFloat(pos.LiquidationPx)

		out = append(out, domain.OpenPosition{
			Pair:             pos.Coin + "USDC",
			Amount:           math.Abs(helpers.Round8(size)),
			Leverage:         helpers.Round8(pos.Leverage.Value),
			EntryPrice:       helpers.Round8(entry),
			Pnl:              helpers.Round8(pnl),
			LiquidationPrice: helpers.Round8(liq),
			CurrentPrice:     helpers.Round8(coinPrice[pos.Coin]),
		})
	}

	return out, nil
}
