package executors

import (
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/connector/hyperliquid/models"
	"net/http"
)

type l2BookRequest struct {
	Type string `json:"type"`
	Coin string `json:"coin"`
}

func FetchL2Book(
	client *http.Client,
	endpoint string,
	coin string,
) (models.L2Book, error) {
	var out models.L2Book
	payload := l2BookRequest{
		Type: "l2Book",
		Coin: coin,
	}

	err := hyperliquid.DoRequest(client, endpoint, payload, &out)
	if err != nil {
		return models.L2Book{}, err
	}

	return out, nil
}
