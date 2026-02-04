package hyperliquid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

func DoRequest(client *http.Client, endpoint string, payload any, out any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("hyperliquid: unexpected status %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
