package blofin

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	BaseURL = "https://openapi.blofin.com"

	defaultTimeout      = 15 * time.Second
	defaultRetryCount   = 2
	defaultRetryWait    = 300 * time.Millisecond
	defaultRetryMaxWait = 2 * time.Second
)

type Credentials struct {
	APIKey     string
	Secret     string
	Passphrase string
}

func NewClient(creds Credentials) *resty.Client {
	client := NewBaseClient()
	AttachAuth(client, creds)
	return client
}

func NewBaseClient() *resty.Client {
	client := resty.New().
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

type APIResponse[T any] struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPError) Error() string {
	if strings.TrimSpace(e.Body) == "" {
		return fmt.Sprintf("blofin: unexpected status %s", e.Status)
	}
	return fmt.Sprintf("blofin: unexpected status %s: %s", e.Status, strings.TrimSpace(e.Body))
}

type APIError struct {
	Code string
	Msg  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("blofin: error %s: %s", e.Code, e.Msg)
}

func DoGet[T any](client *resty.Client, path string, params map[string]string) (T, error) {
	var result APIResponse[T]
	var zero T

	req := client.R().SetResult(&result)
	for k, v := range params {
		if v != "" {
			req.SetQueryParam(k, v)
		}
	}

	resp, err := req.Get(BaseURL + path)
	if err != nil {
		return zero, err
	}

	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusMultipleChoices {
		return zero, &HTTPError{
			StatusCode: resp.StatusCode(),
			Status:     resp.Status(),
			Body:       string(resp.Body()),
		}
	}

	if result.Code != "0" {
		return zero, &APIError{Code: result.Code, Msg: result.Msg}
	}

	return result.Data, nil
}
