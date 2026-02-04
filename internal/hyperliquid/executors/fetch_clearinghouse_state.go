package executors

import (
	"hyperliquid-trade-reconstructor/internal/hyperliquid"
	"hyperliquid-trade-reconstructor/internal/hyperliquid/models"
	"net/http"
)

type clearinghouseStateRequest struct {
	Type string `json:"type"`
	User string `json:"user"`
}

func FetchClearinghouseState(
	client *http.Client,
	endpoint string,
	user string,
) (models.ClearinghouseState, error) {
	var out models.ClearinghouseState

	payload := clearinghouseStateRequest{
		Type: "clearinghouseState",
		User: user,
	}

	err := hyperliquid.DoRequest(client, endpoint, payload, &out)
	if err != nil {
		return models.ClearinghouseState{}, err
	}

	return out, nil
}
