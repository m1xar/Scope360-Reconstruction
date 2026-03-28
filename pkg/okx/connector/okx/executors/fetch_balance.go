package executors

import (
	"github.com/go-resty/resty/v2"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
)

const balancePath = "/api/v5/account/balance"

func FetchBalance(client *resty.Client, baseURL string) (models.Balance, error) {
	data, err := okx.DoGet[[]models.Balance](client, baseURL, balancePath, nil)
	if err != nil {
		return models.Balance{}, err
	}
	if len(data) == 0 {
		return models.Balance{}, nil
	}
	return data[0], nil
}
