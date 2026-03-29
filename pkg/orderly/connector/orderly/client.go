package orderly

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	defaultTimeout = 20 * time.Second
	MainnetBaseURL = "https://api.orderly.org"
	TestnetBaseURL = "https://testnet-api.orderly.org"
)

type Client struct {
	http    *resty.Client
	baseURL string
	creds   Credentials
}

func NewClient(baseURL string, creds Credentials, httpClient *resty.Client) *Client {
	if httpClient == nil {
		httpClient = resty.New().SetTimeout(defaultTimeout)
	}
	return &Client{
		http:    httpClient,
		baseURL: strings.TrimRight(baseURL, "/"),
		creds:   creds,
	}
}

func (c *Client) Get(path string, params map[string]string, out any) error {
	query := buildQueryString(params)
	fullPath := path
	if query != "" {
		fullPath = path + "?" + query
	}

	headers := BuildAuthHeaders(c.creds, "GET", fullPath, "")

	req := c.http.R().
		SetHeaders(headers).
		SetHeader("Content-Type", "application/x-www-form-urlencoded")

	resp, err := req.Get(c.baseURL + fullPath)
	if err != nil {
		return fmt.Errorf("orderly GET %s: %w", path, err)
	}

	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusMultipleChoices {
		return fmt.Errorf("orderly GET %s: unexpected status %d: %s", path, resp.StatusCode(), resp.String())
	}

	if out == nil {
		return nil
	}

	return json.Unmarshal(resp.Body(), out)
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
