package executors

import (
	"hyperliquid-trade-reconstructor/internal/connector/hyperliquid"
	"hyperliquid-trade-reconstructor/internal/connector/hyperliquid/models"
	"net/http"
)

type portfolioStateRequest struct {
	Type string `json:"type"`
	User string `json:"user"`
}

func FetchPortfolioState(
	client *http.Client,
	endpoint string,
	user string,
) (models.RawPortfolioResponse, error) {
	var out models.RawPortfolioResponse

	payload := portfolioStateRequest{
		Type: "portfolio",
		User: user,
	}

	err := hyperliquid.DoRequest(client, endpoint, payload, &out)
	if err != nil {
		return models.RawPortfolioResponse{}, err
	}

	return out, nil
}
