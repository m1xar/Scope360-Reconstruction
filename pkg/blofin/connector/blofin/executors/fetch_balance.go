package executors

import (
	"github.com/go-resty/resty/v2"
	blofin "github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
)

func FetchBalance(client *resty.Client) (models.Balance, error) {
	return blofin.DoGet[models.Balance](
		client,
		"/api/v1/account/balance",
		nil,
	)
}
