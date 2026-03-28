package executors

import (
	"fmt"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
)

const (
	depositHistoryPath    = "/api/v5/asset/deposit-history"
	withdrawalHistoryPath = "/api/v5/asset/withdrawal-history"
	depositsPageLimit     = 100
)

func FetchAllDeposits(client *resty.Client, baseURL string) ([]models.Deposit, error) {
	var result []models.Deposit
	after := ""

	for {
		params := map[string]string{"limit": fmt.Sprintf("%d", depositsPageLimit)}
		if after != "" {
			params["after"] = after
		}

		page, err := okx.DoGet[[]models.Deposit](client, baseURL, depositHistoryPath, params)
		if err != nil {
			if after != "" && isHTTP5xx(err) {
				break
			}
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		result = append(result, page...)
		if len(page) < depositsPageLimit {
			break
		}
		after = page[len(page)-1].DepId
	}

	return result, nil
}

func FetchAllWithdrawals(client *resty.Client, baseURL string) ([]models.Withdrawal, error) {
	var result []models.Withdrawal
	after := ""

	for {
		params := map[string]string{"limit": fmt.Sprintf("%d", depositsPageLimit)}
		if after != "" {
			params["after"] = after
		}

		page, err := okx.DoGet[[]models.Withdrawal](client, baseURL, withdrawalHistoryPath, params)
		if err != nil {
			if after != "" && isHTTP5xx(err) {
				break
			}
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		result = append(result, page...)
		if len(page) < depositsPageLimit {
			break
		}
		after = page[len(page)-1].WdId
	}

	return result, nil
}
