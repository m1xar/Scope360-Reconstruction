package executors

import (
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
)

const billsArchivePath = "/api/v5/account/bills-archive"

func FetchAllBills(client *resty.Client, baseURL string, billType string) ([]models.Bill, error) {
	return FetchAllSwapAndFuturesBills(client, baseURL, billType)
}

func FetchAllBillsByInstType(client *resty.Client, baseURL, instType, billType string) ([]models.Bill, error) {
	var result []models.Bill
	after := ""

	for {
		params := map[string]string{
			"instType": instType,
		}
		if billType != "" {
			params["type"] = billType
		}
		if after != "" {
			params["after"] = after
		}

		page, err := okx.DoGet[[]models.Bill](client, baseURL, billsArchivePath, params)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		result = append(result, page...)
		after = page[len(page)-1].BillId
	}

	return result, nil
}

func FetchAllSwapAndFuturesBills(client *resty.Client, baseURL, billType string) ([]models.Bill, error) {
	var swapBills, futuresBills []models.Bill
	var swapErr, futuresErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		swapBills, swapErr = FetchAllBillsByInstType(client, baseURL, "SWAP", billType)
	}()
	go func() {
		defer wg.Done()
		futuresBills, futuresErr = FetchAllBillsByInstType(client, baseURL, "FUTURES", billType)
	}()
	wg.Wait()

	if swapErr != nil {
		return nil, swapErr
	}
	if futuresErr != nil {
		return nil, futuresErr
	}

	return append(swapBills, futuresBills...), nil
}

func FetchFundingBills(client *resty.Client, baseURL string) ([]models.Bill, error) {
	return FetchAllBills(client, baseURL, "8")
}
