package orderly

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

type Config struct {
	WalletAddress  string
	BrokerID       string
	Ed25519PubKey  string
	Ed25519PrivKey ed25519.PrivateKey
	HTTPClient     *resty.Client
}

const (
	defaultTimeout    = 20 * time.Second
	orderlyBaseURL    = "https://api.orderly.org"
	maxAttempts       = 3
	rateLimitFallback = 10 * time.Second
)

type Client struct {
	http    *resty.Client
	baseURL string
	creds   Credentials
}

func NewClient(cfg Config) *Client {
	creds := NewCredentials(
		cfg.WalletAddress,
		cfg.BrokerID,
		cfg.Ed25519PubKey,
		cfg.Ed25519PrivKey,
	)

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = resty.New().SetTimeout(defaultTimeout)
	}

	return &Client{
		http:    httpClient,
		baseURL: orderlyBaseURL,
		creds:   creds,
	}
}

func (c *Client) Get(path string, params map[string]string, out any) error {
	return c.get(path, params, out, true)
}

func (c *Client) GetPublic(path string, params map[string]string, out any) error {
	return c.get(path, params, out, false)
}

func (c *Client) get(path string, params map[string]string, out any, auth bool) error {
	query := buildQueryString(params)
	fullPath := path
	if query != "" {
		fullPath = path + "?" + query
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req := c.http.R().
			SetHeader("Content-Type", "application/x-www-form-urlencoded")
		if auth {
			req.SetHeaders(BuildAuthHeaders(c.creds, "GET", fullPath, ""))
		}

		resp, err := req.Get(c.baseURL + fullPath)
		if err != nil {
			lastErr = fmt.Errorf("orderly GET %s: %w", path, err)
			if attempt < maxAttempts {
				time.Sleep(retryDelay(attempt))
				continue
			}
			return lastErr
		}

		if resp.StatusCode() >= http.StatusOK && resp.StatusCode() < http.StatusMultipleChoices {
			if out == nil {
				return nil
			}
			return json.Unmarshal(resp.Body(), out)
		}

		lastErr = fmt.Errorf("orderly GET %s: unexpected status %d: %s", path, resp.StatusCode(), resp.String())
		if attempt < maxAttempts && isRetryableStatus(resp.StatusCode()) {
			time.Sleep(statusRetryDelay(resp, attempt))
			continue
		}

		return lastErr
	}

	return lastErr
}

func buildQueryString(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}

	values := url.Values{}
	for k, v := range params {
		if v != "" {
			values.Set(k, v)
		}
	}

	return values.Encode()
}

func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusRequestTimeout,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func statusRetryDelay(resp *resty.Response, attempt int) time.Duration {
	if resp.StatusCode() == http.StatusTooManyRequests {
		if delay, ok := parseRetryAfter(resp.Header().Get("Retry-After")); ok {
			return delay
		}
		return rateLimitFallback
	}
	return retryDelay(attempt)
}

func retryDelay(attempt int) time.Duration {
	return time.Duration(attempt) * 250 * time.Millisecond
}

func parseRetryAfter(value string) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	if seconds, err := strconv.ParseFloat(value, 64); err == nil {
		if seconds < 0 {
			return 0, true
		}
		return time.Duration(seconds * float64(time.Second)), true
	}

	if retryAt, err := http.ParseTime(value); err == nil {
		delay := time.Until(retryAt)
		if delay < 0 {
			delay = 0
		}
		return delay, true
	}

	return 0, false
}
