package kraken

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	BaseURL = "https://futures.kraken.com"

	defaultTimeout      = 20 * time.Second
	defaultRetryCount   = 2
	defaultRetryWait    = 300 * time.Millisecond
	defaultRetryMaxWait = 1 * time.Second
	defaultRateWait     = 2 * time.Second
)

var lastNonce int64

type Credentials struct {
	APIKey string
	Secret string
}

type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
	Header     http.Header
}

func (e *HTTPError) Error() string {
	if strings.TrimSpace(e.Body) == "" {
		return fmt.Sprintf("kraken: unexpected status %s", e.Status)
	}
	return fmt.Sprintf("kraken: unexpected status %s: %s", e.Status, strings.TrimSpace(e.Body))
}

type APIError struct {
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("kraken: %s", e.Message)
}

func NewClient(creds Credentials) *resty.Client {
	client := NewPublicClient()
	AttachAuth(client, creds)
	return client
}

func NewPublicClient() *resty.Client {
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

func AttachAuth(client *resty.Client, creds Credentials) {
	client.SetPreRequestHook(func(_ *resty.Client, req *http.Request) error {
		nonce := nextNonce()
		postData := ""
		if req.Method == http.MethodGet || req.Method == http.MethodDelete {
			postData = req.URL.RawQuery
		}

		authent, err := sign(creds.Secret, req.URL.EscapedPath(), postData, nonce)
		if err != nil {
			return err
		}

		req.Header.Set("APIKey", creds.APIKey)
		req.Header.Set("Authent", authent)
		req.Header.Set("Nonce", nonce)
		req.Header.Set("Accept", "application/json")
		return nil
	})
}

func nextNonce() string {
	now := time.Now().UnixMilli()
	for {
		prev := atomic.LoadInt64(&lastNonce)
		next := now
		if next <= prev {
			next = prev + 1
		}
		if atomic.CompareAndSwapInt64(&lastNonce, prev, next) {
			return fmt.Sprintf("%d", next)
		}
	}
}

func sign(secret, requestPath, postData, nonce string) (string, error) {
	signPath := requestPath
	if strings.HasPrefix(signPath, "/derivatives") {
		signPath = strings.TrimPrefix(signPath, "/derivatives")
	}

	secretBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("kraken: decode api secret: %w", err)
	}

	digest := sha256.Sum256([]byte(postData + nonce + signPath))
	mac := hmac.New(sha512.New, secretBytes)
	if _, err := mac.Write(digest[:]); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

func DoGet[T any](client *resty.Client, path string, params map[string]string) (T, error) {
	var out T
	var zero T

	req := client.R().SetResult(&out)
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
			Header:     resp.Header(),
		}
	}

	if err := checkAPIError(resp.Body()); err != nil {
		return zero, err
	}

	return out, nil
}

func DoGetWithRateLimitRetry[T any](client *resty.Client, path string, params map[string]string, attempts int) (T, error) {
	if attempts < 1 {
		attempts = 1
	}

	var zero T
	for attempt := 1; attempt <= attempts; attempt++ {
		out, err := DoGet[T](client, path, params)
		if err == nil {
			return out, nil
		}
		if attempt == attempts || !IsRateLimitError(err) {
			return zero, err
		}
		time.Sleep(RateLimitWait(err))
	}
	return zero, nil
}

func IsRateLimitError(err error) bool {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode == http.StatusTooManyRequests {
			return true
		}
		if strings.Contains(strings.ToLower(httpErr.Body), "apilimitexceeded") {
			return true
		}
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		msg := strings.ToLower(apiErr.Message)
		return strings.Contains(msg, "apilimitexceeded") || strings.Contains(msg, "rate limit")
	}
	return false
}

func RateLimitWait(err error) time.Duration {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) && httpErr.Header != nil {
		if wait, ok := parseWaitHeader(httpErr.Header.Get("Retry-After")); ok {
			return wait
		}
		if wait, ok := parseWaitHeader(httpErr.Header.Get("rate-limit-reset")); ok {
			return wait
		}
	}
	return defaultRateWait
}

func parseWaitHeader(raw string) (time.Duration, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	if sec, err := strconv.ParseFloat(raw, 64); err == nil {
		if sec > 1_000_000_000 {
			wait := time.Until(time.Unix(int64(sec), 0))
			if wait > 0 {
				return wait, true
			}
			return defaultRetryWait, true
		}
		return time.Duration(sec * float64(time.Second)), true
	}
	if at, err := http.ParseTime(raw); err == nil {
		wait := time.Until(at)
		if wait > 0 {
			return wait, true
		}
		return defaultRetryWait, true
	}
	return 0, false
}

func checkAPIError(body []byte) error {
	var envelope struct {
		Result string `json:"result"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil
	}
	if strings.EqualFold(envelope.Result, "error") {
		msg := envelope.Error
		if msg == "" {
			msg = "api returned result=error"
		}
		return &APIError{Message: msg}
	}
	return nil
}
