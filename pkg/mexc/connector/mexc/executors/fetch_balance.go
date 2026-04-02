package executors

import (
	"github.com/go-resty/resty/v2"
	mexc "github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
)

const assetsPath = "/api/v1/private/account/assets"

func FetchAssets(client *resty.Client) ([]models.Asset, error) {
	return mexc.DoGet[[]models.Asset](client, assetsPath, nil)
}

const assetPath = "/api/v1/private/account/asset/USDT"

func FetchUSDTAsset(client *resty.Client) (models.Asset, error) {
	return mexc.DoGet[models.Asset](client, assetPath, nil)
}
