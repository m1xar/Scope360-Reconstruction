package executors

import (
	"fmt"

	"github.com/go-resty/resty/v2"
	kraken "github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
)

const positionEventsPageSize = 1000

func FetchPositionEventsPageDesc(client *resty.Client, tradeable, continuation string) (models.PositionEventsResponse, error) {
	params := map[string]string{
		"sort":      "desc",
		"count":     fmt.Sprintf("%d", positionEventsPageSize),
		"tradeable": tradeable,
	}
	if continuation != "" {
		params["continuation_token"] = continuation
	}
	return kraken.DoGetWithRateLimitRetry[models.PositionEventsResponse](client, positionEventsPath, params, positionEventsRateLimitedTries)
}
