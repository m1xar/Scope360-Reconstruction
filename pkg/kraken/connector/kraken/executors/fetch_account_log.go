package executors

import (
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	kraken "github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken"
	"github.com/m1xar/scope360-reconstruction/pkg/kraken/connector/kraken/models"
)

const (
	accountLogPath             = "/api/history/v3/account-log"
	accountLogPageSize         = 5000
	accountLogRateLimitedTries = 4
)

func FetchAllAccountLog(client *resty.Client, days int) ([]models.AccountLog, error) {
	params := map[string]string{
		"count": fmt.Sprint(accountLogPageSize),
		"sort":  "asc",
	}
	if days > 0 {
		params["since"] = time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)
	}

	var result []models.AccountLog
	var lastID int64
	for {
		if lastID > 0 {
			params["from"] = fmt.Sprint(lastID + 1)
		}

		resp, err := kraken.DoGetWithRateLimitRetry[models.AccountLogResponse](client, accountLogPath, params, accountLogRateLimitedTries)
		if err != nil {
			return nil, err
		}
		if len(resp.Logs) == 0 {
			break
		}

		advanced := false
		for _, row := range resp.Logs {
			result = append(result, row)
			if row.ID > lastID {
				lastID = row.ID
				advanced = true
			}
		}
		if len(resp.Logs) < accountLogPageSize || !advanced {
			break
		}
	}

	return result, nil
}
