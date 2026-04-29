package executors

import (
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	kraken "github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
)

const positionEventsPath = "/api/history/v3/positions"

func FetchAllPositionEvents(client *resty.Client, days int) ([]models.PositionEventElement, error) {
	params := map[string]string{
		"sort": "asc",
	}
	if days > 0 {
		params["since"] = fmt.Sprint(time.Now().AddDate(0, 0, -days).UnixMilli())
	}

	var result []models.PositionEventElement
	token := ""
	for {
		if token != "" {
			params["continuation_token"] = token
		} else {
			delete(params, "continuation_token")
		}

		resp, err := kraken.DoGet[models.PositionEventsResponse](client, positionEventsPath, params)
		if err != nil {
			return nil, err
		}
		if len(resp.Elements) == 0 {
			break
		}
		result = append(result, resp.Elements...)
		if resp.ContinuationToken == "" {
			break
		}
		token = resp.ContinuationToken
	}

	return result, nil
}
