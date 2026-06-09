package executors

import (
	"strconv"

	"github.com/go-resty/resty/v2"
	blofin "github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
)

func FetchAllOrdersHistory(client *resty.Client, beginMs int64) ([]models.Order, error) {
	params := map[string]string{"limit": strconv.Itoa(defaultLimit)}
	if beginMs > 0 {
		params["begin"] = strconv.FormatInt(beginMs, 10)
	}
	return fetchOrderPages(client, "/api/v1/trade/orders-history", params, beginMs)
}

func FetchAllTPSLOrdersHistory(client *resty.Client, beginMs int64) ([]models.Order, error) {
	params := map[string]string{"limit": strconv.Itoa(defaultLimit)}
	return fetchOrderPages(client, "/api/v1/trade/orders-tpsl-history", params, beginMs)
}

func FetchAllAlgoOrdersHistory(client *resty.Client, beginMs int64) ([]models.Order, error) {
	params := map[string]string{"limit": strconv.Itoa(defaultLimit)}
	return fetchOrderPages(client, "/api/v1/trade/orders-algo-history", params, beginMs)
}

func fetchOrderPages(
	client *resty.Client,
	path string,
	params map[string]string,
	beginMs int64,
) ([]models.Order, error) {
	var all []models.Order
	for {
		page, err := blofin.DoGet[[]models.Order](client, path, params)
		if err != nil {
			return nil, err
		}

		oldestInPage := int64(0)
		for _, ord := range page {
			t := orderTimeMs(ord)
			if t > 0 && (oldestInPage == 0 || t < oldestInPage) {
				oldestInPage = t
			}
			if beginMs <= 0 || t == 0 || t >= beginMs {
				all = append(all, ord)
			}
		}

		if len(page) < defaultLimit || (beginMs > 0 && oldestInPage > 0 && oldestInPage < beginMs) {
			break
		}

		cursor := orderCursor(page[len(page)-1])
		if cursor == "" {
			break
		}
		params["after"] = cursor
	}
	return all, nil
}

func orderCursor(ord models.Order) string {
	if ord.OrderID != "" {
		return ord.OrderID
	}
	if ord.TPSLID != "" {
		return ord.TPSLID
	}
	return ord.AlgoID
}

func orderTimeMs(ord models.Order) int64 {
	if ord.UpdateTime != "" {
		return models.Int64(ord.UpdateTime)
	}
	return models.Int64(ord.CreateTime)
}
