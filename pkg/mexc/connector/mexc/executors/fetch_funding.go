package executors

import (
	"fmt"

	"github.com/go-resty/resty/v2"
	mexc "github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
)

const fundingRecordsPath = "/api/v1/private/position/funding_records"

const fundingPageSize = 100

func FetchAllFundingRecords(client *resty.Client) ([]models.FundingRecord, error) {
	var result []models.FundingRecord
	page := 1

	for {
		params := map[string]string{
			"page_num":  fmt.Sprint(page),
			"page_size": fmt.Sprint(fundingPageSize),
		}

		data, err := mexc.DoGet[[]models.FundingRecord](client, fundingRecordsPath, params)
		if err != nil {
			if page > 1 && isHTTP5xx(err) {
				break
			}
			return nil, err
		}
		if len(data) == 0 {
			break
		}

		result = append(result, data...)

		if len(data) < fundingPageSize {
			break
		}
		page++
	}

	return result, nil
}
