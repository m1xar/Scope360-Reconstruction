package builders

import (
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/service/reconstructor/helpers"
)

func BuildOpenPosition(pos models.OpenPosition, ticker models.Ticker) domain.OpenPosition {
	openTime, _ := helpers.ParseTime(pos.FillTime)
	side := "LONG"
	if strings.EqualFold(pos.Side, "short") {
		side = "SHORT"
	}

	pair := ticker.Pair
	if pair == "" {
		pair = helpers.NormalizePairFallback(pos.Symbol)
	}

	return domain.OpenPosition{
		Pair:         strings.ToUpper(strings.ReplaceAll(pair, "_", "")),
		Amount:       helpers.Round8(pos.Size.Float64()),
		Side:         side,
		EntryPrice:   helpers.Round8(pos.Price.Float64()),
		CurrentPrice: helpers.Round8(ticker.MarkPrice.Float64()),
		OpenTime:     openTime,
	}
}
