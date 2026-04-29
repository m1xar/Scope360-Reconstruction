package executors

import (
	"github.com/go-resty/resty/v2"
	kraken "github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
)

const accountsPath = "/derivatives/api/v3/accounts"

func FetchAccounts(client *resty.Client) (models.AccountsResponse, error) {
	return kraken.DoGet[models.AccountsResponse](client, accountsPath, nil)
}
