package executors

import (
	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
)

type clearinghouseStateRequest struct {
	Type string `json:"type"`
	User string `json:"user"`
}

func FetchClearinghouseState(
	client *resty.Client,
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
