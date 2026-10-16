package executors

import (
	"context"
	"net/http"
	"net/url"

	connector "github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade"
	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
)

func FetchInstrumentList(ctx context.Context, c *connector.Client) ([]models.Instrument, error) {
	var list models.InstrumentList
	query := url.Values{"symbols": {"*"}}
	if err := c.Do(ctx, "instrument_list", http.MethodGet, "/instruments/query", query, nil, &list); err != nil {
		return nil, err
	}
	return list.Instruments, nil
}

func FetchInstruments(ctx context.Context, c *connector.Client) ([]models.InstrumentDetails, error) {
	path, err := c.AccountPath(ctx, "/instruments/query")
	if err != nil {
		return nil, err
	}
	var list models.InstrumentDetailsList
	query := url.Values{"symbols": {"*"}}
	if err := c.Do(ctx, "instruments", http.MethodGet, path, query, nil, &list); err != nil {
		return nil, err
	}
	return list.InstrumentDetails, nil
}
