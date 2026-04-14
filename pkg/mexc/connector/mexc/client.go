package mexc

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	BaseURL = "https://contract.mexc.com"

	defaultTimeout      = 15 * time.Second
	defaultRetryCount   = 2
	defaultRetryWait    = 300 * time.Millisecond
	defaultRetryMaxWait = 1 * time.Second
)

type Credentials struct {
	APIKey string
	Secret string
}

func AttachAuth(client *resty.Client, creds Credentials) {
	client.SetPreRequestHook(func(_ *resty.Client, req *http.Request) error {
		ts := fmt.Sprintf("%d", time.Now().UnixMilli())

		var paramStr string
		if req.Method == http.MethodGet || req.Method == http.MethodDelete {
			paramStr = sortedQueryString(req)
		}

		signature := sign(creds.APIKey, creds.Secret, ts, paramStr)

		req.Header.Set("ApiKey", creds.APIKey)
		req.Header.Set("Request-Time", ts)
		req.Header.Set("Signature", signature)
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

func sign(apiKey, secret, timestamp, paramString string) string {
	payload := apiKey + timestamp + paramString
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

func sortedQueryString(req *http.Request) string {
	query := req.URL.Query()
	if len(query) == 0 {
		return ""
	}

	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		v := query.Get(k)
		if v != "" {
			pairs = append(pairs, k+"="+v)
		}
	}
	return strings.Join(pairs, "&")
}

type APIResponse[T any] struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Data    T      `json:"data"`
	Message string `json:"message"`
}

type PageResponse[T any] struct {
	Success bool `json:"success"`
	Code    int  `json:"code"`
	Data    []T  `json:"data"`
}

// PaginatedData is the wrapper for endpoints that return {resultList, totalCount, ...}.
type PaginatedData[T any] struct {
	ResultList  []T `json:"resultList"`
	TotalCount  int `json:"totalCount"`
	TotalPage   int `json:"totalPage"`
	CurrentPage int `json:"currentPage"`
	PageSize    int `json:"pageSize"`
}

// DoGetPaginated is like DoGet but for endpoints that wrap data in {resultList: [...]}.
func DoGetPaginated[T any](client *resty.Client, path string, params map[string]string) ([]T, error) {
	page, err := DoGet[PaginatedData[T]](client, path, params)
	if err != nil {
		return nil, err
	}
	return page.ResultList, nil
}

type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPError) Error() string {
	if strings.TrimSpace(e.Body) == "" {
		return fmt.Sprintf("mexc: unexpected status %s", e.Status)
	}
	return fmt.Sprintf("mexc: unexpected status %s: %s", e.Status, strings.TrimSpace(e.Body))
}

type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("mexc: error %d: %s", e.Code, e.Message)
}

func DoGet[T any](client *resty.Client, path string, params map[string]string) (T, error) {
	var result APIResponse[T]
	var zero T

	req := client.R().SetResult(&result)
	for k, v := range params {
		req.SetQueryParam(k, v)
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

	if !result.Success {
		return zero, &APIError{Code: result.Code, Message: result.Message}
	}

	return result.Data, nil
}
