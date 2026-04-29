package executors

import (
	"net/url"

	"github.com/go-resty/resty/v2"
	kraken "github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
)

const tickersPath = "/derivatives/api/v3/tickers"

func FetchTickers(client *resty.Client) ([]models.Ticker, error) {
	resp, err := kraken.DoGet[models.TickersResponse](client, tickersPath, nil)
	if err != nil {
		return nil, err
	}
	return resp.Tickers, nil
}

func FetchTicker(client *resty.Client, symbol string) (models.Ticker, error) {
	resp, err := kraken.DoGet[models.TickerResponse](client, tickersPath+"/"+url.PathEscape(symbol), nil)
	if err != nil {
		return models.Ticker{}, err
	}
	return resp.Ticker, nil
}
