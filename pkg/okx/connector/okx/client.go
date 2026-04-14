package okx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	defaultTimeout      = 10 * time.Second
	defaultRetryCount   = 2
	defaultRetryWait    = 300 * time.Millisecond
	defaultRetryMaxWait = 1 * time.Second
)

type Credentials struct {
	APIKey     string
	Secret     string
	Passphrase string
}

func AttachAuth(client *resty.Client, creds Credentials) {
	client.SetPreRequestHook(func(_ *resty.Client, req *http.Request) error {
		ts := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		method := req.Method
		path := req.URL.RequestURI()

		sign := signRequest(creds.Secret, ts, method, path, "")

		req.Header.Set("OK-ACCESS-KEY", creds.APIKey)
		req.Header.Set("OK-ACCESS-SIGN", sign)
		req.Header.Set("OK-ACCESS-TIMESTAMP", ts)
		req.Header.Set("OK-ACCESS-PASSPHRASE", creds.Passphrase)
		req.Header.Set("Content-Type", "application/json")

		return nil
	})
}

func NewClient(creds Credentials) *resty.Client {
	client := newBaseClient()
	AttachAuth(client, creds)
	return client
}

func NewPublicClient() *resty.Client {
	return newBaseClient()
}

func newBaseClient() *resty.Client {
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

func signRequest(secret, timestamp, method, path, body string) string {
	prehash := timestamp + method + path + body
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(prehash))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
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
		return fmt.Sprintf("okx: unexpected status %s", e.Status)
	}
	return fmt.Sprintf("okx: unexpected status %s: %s", e.Status, strings.TrimSpace(e.Body))
}

type APIError struct {
	Code string
	Msg  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("okx: error %s: %s", e.Code, e.Msg)
}

func DoGet[T any](client *resty.Client, baseURL, path string, params map[string]string) (T, error) {
	var result APIResponse[T]
	var zero T

	req := client.R().SetResult(&result)
	for k, v := range params {
		req.SetQueryParam(k, v)
	}

	resp, err := req.Get(baseURL + path)
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
