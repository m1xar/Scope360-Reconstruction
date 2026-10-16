package dxtrade

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
)

const (
	maxRetries     = 3
	requestsPerSec = 8
	defaultTimeout = 60 * time.Second
)

type Client struct {
	cfg   Config
	host  string
	http  *http.Client
	rate  *limiter
	login *limiter

	loginMu sync.Mutex
	mu      sync.Mutex
	token   string
	domain  string
	timeout time.Duration
	lastUse time.Time

	accountMu sync.Mutex
	account   *models.AccountDetails
}

type APIError struct {
	Status      int
	Code        string
	Description string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("dxtrade: http %d: %s %s", e.Status, e.Code, e.Description)
}

func IsStatus(err error, status int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == status
}

func NewClient(cfg Config) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{
		cfg:   cfg,
		host:  cfg.host(),
		http:  httpClient,
		rate:  newLimiter(requestsPerSec, requestsPerSec),
		login: newLimiter(1, 1),
	}
}

func (c *Client) Do(ctx context.Context, name, method, path string, query url.Values, body, out any) error {
	token, err := c.ensureSession(ctx)
	if err != nil {
		return err
	}
	return c.call(ctx, name, method, c.host+apiPrefix+path, query, body, out, token)
}

func (c *Client) call(ctx context.Context, name, method, rawURL string, query url.Values, body, out any, token string) error {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return err
		}
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	relogged := false
	for attempt := 0; ; attempt++ {
		if err := c.rate.Wait(ctx); err != nil {
			return err
		}
		status, header, resp, err := c.send(ctx, name, method, u, payload, token)
		if err != nil {
			return err
		}
		if status == http.StatusUnauthorized && token != "" && !relogged {
			relogged = true
			if token, err = c.relogin(ctx, token); err != nil {
				return err
			}
			continue
		}
		if (status == http.StatusTooManyRequests || status >= 500) && attempt < maxRetries {
			if err := sleepCtx(ctx, backoff(header, attempt)); err != nil {
				return err
			}
			continue
		}
		if status < 200 || status >= 300 {
			return newAPIError(status, resp)
		}
		if token != "" {
			c.mu.Lock()
			c.lastUse = time.Now()
			c.mu.Unlock()
		}
		if out == nil || len(bytes.TrimSpace(resp)) == 0 {
			return nil
		}
		if err := json.Unmarshal(resp, out); err != nil {
			return fmt.Errorf("dxtrade %s: decode: %w", name, err)
		}
		return nil
	}
}

func (c *Client) send(ctx context.Context, name, method string, u *url.URL, payload []byte, token string) (int, http.Header, []byte, error) {
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "DXAPI "+token)
	}
	start := time.Now()
	res, err := c.http.Do(req)
	ex := Exchange{Name: name, Method: method, Path: u.RequestURI(), RequestBody: payload}
	if err != nil {
		ex.Duration, ex.Err = time.Since(start), err
		c.record(ex)
		return 0, nil, nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	ex.Status, ex.ResponseBody, ex.Duration, ex.Err = res.StatusCode, body, time.Since(start), err
	c.record(ex)
	if err != nil {
		return 0, nil, nil, err
	}
	return res.StatusCode, res.Header, body, nil
}

func backoff(header http.Header, attempt int) time.Duration {
	if v := strings.TrimSpace(header.Get("Retry-After")); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
		if at, err := http.ParseTime(v); err == nil {
			return time.Until(at)
		}
	}
	return time.Duration(500<<attempt) * time.Millisecond
}

func newAPIError(status int, body []byte) error {
	apiErr := &APIError{Status: status}
	var payload models.APIError
	if json.Unmarshal(body, &payload) == nil {
		apiErr.Code, apiErr.Description = payload.ErrorCode, payload.Description
	}
	if apiErr.Description == "" && apiErr.Code == "" {
		text := strings.TrimSpace(string(body))
		if len(text) > 200 {
			text = text[:200]
		}
		apiErr.Description = text
	}
	return apiErr
}

func EscapeSegment(s string) string {
	return strings.ReplaceAll(url.PathEscape(s), ":", "%3A")
}
