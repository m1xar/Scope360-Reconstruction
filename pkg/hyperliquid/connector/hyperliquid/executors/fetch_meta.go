package executors

import (
	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
)

type metaRequest struct {
	Type string `json:"type"`
}

func FetchMeta(client *resty.Client, endpoint string) (models.MetaResponse, error) {
	var out models.MetaResponse
	if err := hyperliquid.DoRequest(client, endpoint, metaRequest{Type: "meta"}, &out); err != nil {
		return models.MetaResponse{}, err
	}
	return out, nil
}
