package builders

import (
	"hyperliquid-trade-reconstructor/internal/connector/hyperliquid/executors"
	"hyperliquid-trade-reconstructor/internal/connector/hyperliquid/models"
	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/service/reconstructor/helpers"
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
			Pair:             pos.Coin + "/USDC",
			Amount:           size,
			Leverage:         pos.Leverage.Value,
			EntryPrice:       entry,
			Pnl:              pnl,
			LiquidationPrice: liq,
			CurrentPrice:     coinPrice[pos.Coin],
		})
	}

	return out, nil
}
