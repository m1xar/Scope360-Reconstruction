package executors

import (
	"context"
	"encoding/json"
	"net/http"

	connector "github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader"
	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
)

func FetchOpenPositions(ctx context.Context, c *connector.Client) ([]models.OpenPosition, error) {
	var raw json.RawMessage
	if err := c.Do(ctx, http.MethodGet, "open-positions", nil, nil, &raw); err != nil {
		return nil, err
	}
	return decodeList[models.OpenPosition]("open-positions", raw, "positions")
}

func FetchActiveOrders(ctx context.Context, c *connector.Client) ([]models.ActiveOrder, error) {
	var raw json.RawMessage
	if err := c.Do(ctx, http.MethodGet, "active-orders", nil, nil, &raw); err != nil {
		return nil, err
	}
	return decodeList[models.ActiveOrder]("active-orders", raw, "orders")
}

func FetchInstruments(ctx context.Context, c *connector.Client) ([]models.Instrument, error) {
	raw, err := c.Instruments(ctx)
	if err != nil {
		return nil, err
	}
	return decodeList[models.Instrument]("effective-instruments", raw, "instruments", "symbols", "body")
}
