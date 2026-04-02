package executors

import (
	"fmt"

	"github.com/go-resty/resty/v2"
	mexc "github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
)

const transferRecordPath = "/api/v1/private/account/transfer_record"

const transferPageSize = 100

func FetchAllTransferRecords(client *resty.Client) ([]models.TransferRecord, error) {
	var result []models.TransferRecord
	page := 1

	for {
		params := map[string]string{
			"page_num":  fmt.Sprint(page),
			"page_size": fmt.Sprint(transferPageSize),
			"state":     "SUCCESS",
		}

		data, err := mexc.DoGet[[]models.TransferRecord](client, transferRecordPath, params)
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

		if len(data) < transferPageSize {
			break
		}
		page++
	}

	return result, nil
}
