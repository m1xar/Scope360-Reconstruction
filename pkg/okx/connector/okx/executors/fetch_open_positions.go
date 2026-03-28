package executors

import (
	"github.com/go-resty/resty/v2"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
)

const positionsPath = "/api/v5/account/positions"

func FetchOpenPositions(client *resty.Client, baseURL string) ([]models.OpenPosition, error) {
	params := map[string]string{
		"instType": "SWAP",
	}

	swaps, err := okx.DoGet[[]models.OpenPosition](client, baseURL, positionsPath, params)
	if err != nil {
		return nil, err
	}

	params["instType"] = "FUTURES"
	futures, err := okx.DoGet[[]models.OpenPosition](client, baseURL, positionsPath, params)
	if err != nil {
		return nil, err
	}

	return append(swaps, futures...), nil
}
