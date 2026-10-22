package hyperliquid

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	defaultTimeout      = 20 * time.Second
	defaultRetryCount   = 3
	defaultRetryWait    = 500 * time.Millisecond
	defaultRetryMaxWait = 2 * time.Second
)

type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPError) Error() string {
	if strings.TrimSpace(e.Body) == "" {
		return fmt.Sprintf("hyperliquid: unexpected status %s", e.Status)
	}
	return fmt.Sprintf("hyperliquid: unexpected status %s: %s", e.Status, strings.TrimSpace(e.Body))
}

func (e *HTTPError) GetStatusCode() int {
	if e == nil {
		return 0
	}
	return e.StatusCode
}

func NewBaseClient() *resty.Client {
	return ConfigureRetries(resty.New())
}

func ConfigureRetries(client *resty.Client) *resty.Client {
	if client == nil {
		client = resty.New()
	}
	client.
		SetTimeout(defaultTimeout).
		SetRetryCount(defaultRetryCount).
		SetRetryWaitTime(defaultRetryWait).
		SetRetryMaxWaitTime(defaultRetryMaxWait)
	client.AddRetryCondition(func(resp *resty.Response, err error) bool {
		if err != nil {
			return true
		}
		if resp == nil {
			return false
		}
		code := resp.StatusCode()
		return code == http.StatusTooManyRequests || code >= http.StatusInternalServerError
	})
	return client
}

func DoRequest(client *resty.Client, endpoint string, payload any, out any) error {
	if client == nil {
		client = NewBaseClient()
	}

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		Post(endpoint)
	if err != nil {
		return err
	}

	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusMultipleChoices {
		return &HTTPError{
			StatusCode: resp.StatusCode(),
			Status:     resp.Status(),
			Body:       string(resp.Body()),
		}
	}

	if out == nil {
		return nil
	}

	return json.Unmarshal(resp.Body(), out)
}
