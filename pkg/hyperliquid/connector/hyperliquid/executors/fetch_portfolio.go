package executors

import (
	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
)

type portfolioStateRequest struct {
	Type string `json:"type"`
	User string `json:"user"`
}

func FetchPortfolioState(
	client *resty.Client,
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
