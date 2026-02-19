package hyperliquid

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-resty/resty/v2"
)

func DoRequest(client *resty.Client, endpoint string, payload any, out any) error {
	if client == nil {
		client = resty.New()
	}

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		Post(endpoint)
	if err != nil {
		return err
	}

	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusMultipleChoices {
		return fmt.Errorf("hyperliquid: unexpected status %s", resp.Status())
	}

	if out == nil {
		return nil
	}

	return json.Unmarshal(resp.Body(), out)
}
