package executors

import (
	"github.com/go-resty/resty/v2"
	kraken "github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
)

const openPositionsPath = "/derivatives/api/v3/openpositions"

func FetchOpenPositions(client *resty.Client) ([]models.OpenPosition, error) {
	resp, err := kraken.DoGet[models.OpenPositionsResponse](client, openPositionsPath, nil)
	if err != nil {
		return nil, err
	}
	return resp.OpenPositions, nil
}
