package executors

import (
	orderly "github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly"
	"github.com/m1xar/scope360-reconstruction/pkg/orderly/connector/orderly/models"
)

func FetchPositionsSnapshot(client *orderly.Client) (*models.OrderlyPositionsResponse, error) {
	resp, err := fetchSingleResponse[models.OrderlyPositionsResponse](client, "/v1/positions", nil)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func FetchOpenPositions(client *orderly.Client) ([]models.OrderlyPosition, error) {
	resp, err := FetchPositionsSnapshot(client)
	if err != nil {
		return nil, err
	}

	active := make([]models.OrderlyPosition, 0, len(resp.Rows))
	for _, p := range resp.Rows {
		if p.PositionQty != 0 {
			active = append(active, p)
		}
	}

	return active, nil
}
