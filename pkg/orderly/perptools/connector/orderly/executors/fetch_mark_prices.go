package executors

import (
	"fmt"
	"strings"

	orderly "github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/perptools/connector/orderly/models"
)

func FetchMarkPrices(client *orderly.Client) (map[string]float64, error) {
	var resp models.OrderlyResponse[models.OrderlyFuturesResponse]
	if err := client.GetPublic("/v1/public/futures", nil, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("orderly /v1/public/futures: API returned success=false")
	}

	prices := make(map[string]float64, len(resp.Data.Rows))
	for _, market := range resp.Data.Rows {
		if market.MarkPrice <= 0 {
			continue
		}
		for _, token := range []string{
			tokenFromMarketName(market.Symbol),
			tokenFromMarketName(market.DisplaySymbolName),
		} {
			if token != "" {
				prices[token] = market.MarkPrice
			}
		}
	}
	return prices, nil
}

func tokenFromMarketName(symbol string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	symbol = strings.ReplaceAll(symbol, "-", "_")
	symbol = strings.ReplaceAll(symbol, "/", "_")
	symbol = strings.ReplaceAll(symbol, " ", "_")
	symbol = strings.Trim(symbol, "_")
	symbol = strings.TrimPrefix(symbol, "PERP_")
	for _, suffix := range []string{"_PERP", "_USDC", "_USDT", "_USD"} {
		symbol = strings.TrimSuffix(symbol, suffix)
	}
	return symbol
}
