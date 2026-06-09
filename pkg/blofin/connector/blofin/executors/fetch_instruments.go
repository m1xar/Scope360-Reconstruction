package executors

import (
	"github.com/go-resty/resty/v2"
	blofin "github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
)

func FetchAllInstruments(client *resty.Client) ([]models.Instrument, error) {
	return blofin.DoGet[[]models.Instrument](
		client,
		"/api/v1/market/instruments",
		nil,
	)
}
