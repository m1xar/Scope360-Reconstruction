package executors

import (
	"strconv"

	"github.com/go-resty/resty/v2"
	blofin "github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
)

func FetchAllAssetBills(client *resty.Client) ([]models.Bill, error) {
	params := map[string]string{"limit": strconv.Itoa(defaultLimit)}

	var all []models.Bill
	for {
		page, err := blofin.DoGet[[]models.Bill](
			client,
			"/api/v1/asset/bills",
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
		if last.ID != "" {
			params["after"] = last.ID
			continue
		}
		if last.TS == "" {
			break
		}
		params["after"] = last.TS
	}
	return all, nil
}
