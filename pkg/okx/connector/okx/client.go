package okx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

const defaultTimeout = 20 * time.Second

type Credentials struct {
	APIKey     string
	Secret     string
	Passphrase string
}

func NewClient(creds Credentials) *resty.Client {
	client := resty.New().SetTimeout(defaultTimeout)

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
		return zero, fmt.Errorf("okx: unexpected status %s", resp.Status())
	}

	if result.Code != "0" {
		return zero, fmt.Errorf("okx: error %s: %s", result.Code, result.Msg)
	}

	return result.Data, nil
}
