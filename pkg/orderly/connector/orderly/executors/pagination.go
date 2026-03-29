package executors

import (
	"fmt"
	"strconv"

	orderly "github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly/models"
)

func fetchAllPaged[T any](client *orderly.Client, path string, baseParams map[string]string, pageSize int) ([]T, error) {
	var all []T
	page := 1

	for {
		params := make(map[string]string, len(baseParams)+2)
		for k, v := range baseParams {
			params[k] = v
		}
		params["page"] = strconv.Itoa(page)
		params["size"] = strconv.Itoa(pageSize)

		var resp models.OrderlyResponse[models.PagedData[T]]
		if err := client.Get(path, params, &resp); err != nil {
			return nil, fmt.Errorf("fetchAllPaged %s page %d: %w", path, page, err)
		}

		if !resp.Success {
			return nil, fmt.Errorf("fetchAllPaged %s: API returned success=false", path)
		}

		all = append(all, resp.Data.Rows...)

		if len(all) >= resp.Data.Meta.Total || len(resp.Data.Rows) == 0 {
			break
		}

		page++
	}

	return all, nil
}

func fetchSingleResponse[T any](client *orderly.Client, path string, params map[string]string) (T, error) {
	var resp models.OrderlyResponse[T]
	var zero T

	if err := client.Get(path, params, &resp); err != nil {
		return zero, err
	}

	if !resp.Success {
		return zero, fmt.Errorf("orderly %s: API returned success=false", path)
	}

	return resp.Data, nil
}
