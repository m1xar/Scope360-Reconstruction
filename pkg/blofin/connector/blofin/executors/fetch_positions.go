package executors

import (
	"strconv"

	"github.com/go-resty/resty/v2"
	blofin "github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
)

const defaultLimit = 100

func FetchOpenPositions(client *resty.Client) ([]models.OpenPosition, error) {
	return blofin.DoGet[[]models.OpenPosition](
		client,
		"/api/v1/account/positions",
		nil,
	)
}

func FetchAllPositionHistory(client *resty.Client) ([]models.PositionHistory, error) {
	return FetchAllPositionHistoryRange(client, 0, 0)
}

func FetchAllPositionHistoryRange(
	client *resty.Client,
	beginMs int64,
	endMs int64,
) ([]models.PositionHistory, error) {
	params := map[string]string{
		"limit": strconv.Itoa(defaultLimit),
	}
	if beginMs > 0 {
		params["begin"] = strconv.FormatInt(beginMs, 10)
	}
	if endMs > 0 {
		params["end"] = strconv.FormatInt(endMs, 10)
	}

	var all []models.PositionHistory
	for {
		page, err := blofin.DoGet[[]models.PositionHistory](
			client,
			"/api/v1/account/positions-history",
			params,
		)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < defaultLimit {
			break
		}
		last := page[len(page)-1]
		if last.HistoryID == "" {
			break
		}
		params["after"] = last.HistoryID
	}
	return all, nil
}
