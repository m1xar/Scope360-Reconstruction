package executors

import (
	"github.com/go-resty/resty/v2"
	kraken "github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
)

const checkAPIKeyPath = "/api-keys/v3/check"

func CheckAPIKey(client *resty.Client) (models.ResultResponse, error) {
	return kraken.DoGet[models.ResultResponse](client, checkAPIKeyPath, nil)
}
