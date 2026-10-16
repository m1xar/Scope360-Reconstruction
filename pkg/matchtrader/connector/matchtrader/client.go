package matchtrader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	userAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36"
	maxRetries = 3
	coAuth     = "co-auth"

	prefixAPI  = "mtr-api"
	prefixEdge = "trading-edge"
)

var ErrCloudflareChallenge = errors.New("matchtrader: blocked by Cloudflare challenge; the broker must allowlist the egress IP or provide Broker API access")

type HTTPError struct {
	Name   string
	Status int
	Body   string
}

func (e *HTTPError) Error() string {
	body := e.Body
	if len(body) > 200 {
		body = body[:200]
	}
	return fmt.Sprintf("matchtrader %s: HTTP %d: %s", e.Name, e.Status, body)
}

func IsStatus(err error, codes ...int) bool {
	var he *HTTPError
	if !errors.As(err, &he) {
		return false
	}
	for _, c := range codes {
		if he.Status == c {
			return true
		}
	}
	return false
}

type coAuthMode int

const (
	coAuthLogin coAuthMode = iota
	coAuthAccount
)

type Client struct {
	cfg      Config
	base     string
	http     *http.Client
	jar      http.CookieJar
	recorder Recorder
	limiter  *bucket
	backoff  time.Duration

	authMu  sync.Mutex
	mu      sync.Mutex
	session *Session

	instrumentsMu sync.Mutex
	instruments   json.RawMessage
	mode          coAuthMode
	known         bool
	prefix        string
}

func NewClient(cfg Config) *Client {
	hc := &http.Client{Timeout: 60 * time.Second}
	if cfg.HTTPClient != nil {
		copied := *cfg.HTTPClient
		hc = &copied
	}
	hc.Jar = nil
	jar, _ := cookiejar.New(nil)
	return &Client{
		cfg:      cfg,
		base:     strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		http:     hc,
		jar:      jar,
		recorder: cfg.Recorder,
		limiter:  newBucket(500, time.Minute, 25),
		backoff:  500 * time.Millisecond,
		prefix:   prefixAPI,
	}
}

func (c *Client) Do(ctx context.Context, method, endpoint string, query url.Values, body, out any) error {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return err
		}
	}
	sess, err := c.EnsureSession(ctx)
	if err != nil {
		return err
	}
	c.mu.Lock()
	mode, known, prefix := c.mode, c.known, c.prefix
	c.mu.Unlock()

	triedAlt, triedPrefix, refreshed, relogged := false, false, false, false
	for {
		status, data, err := c.trading(ctx, sess, prefix, mode, method, endpoint, query, payload)
		if err != nil {
			return err
		}
		switch {
		case status >= 200 && status < 300:
			c.mu.Lock()
			c.mode, c.known, c.prefix = mode, true, prefix
			c.mu.Unlock()
			if out == nil || len(bytes.TrimSpace(data)) == 0 {
				return nil
			}
			if err := json.Unmarshal(data, out); err != nil {
				return fmt.Errorf("matchtrader %s: decode: %w", endpoint, err)
			}
			return nil
		case status == http.StatusNotFound && !triedPrefix && prefix == prefixAPI && !known:
			triedPrefix, prefix = true, prefixEdge
			continue
		case status == http.StatusUnauthorized:
			if alt := sess.value(1 - mode); !known && !triedAlt && alt != "" && alt != sess.value(mode) {
				triedAlt, mode = true, 1-mode
				continue
			}
			if !refreshed {
				refreshed = true
				if next, err := c.refresh(ctx, sess); err == nil {
					sess = next
					continue
				}
			}
			if !relogged {
				relogged = true
				next, err := c.relogin(ctx, sess)
				if err != nil {
					return err
				}
				sess, triedAlt = next, false
				continue
			}
		}
		return &HTTPError{Name: endpoint, Status: status, Body: string(data)}
	}
}

func (c *Client) trading(ctx context.Context, sess *Session, prefix string, mode coAuthMode, method, endpoint string, query url.Values, payload []byte) (int, []byte, error) {
	u := c.base + "/" + prefix + "/" + url.PathEscape(sess.SystemUUID) + "/" + strings.TrimLeft(endpoint, "/")
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	header := http.Header{}
	header.Set("Auth-trading-api", sess.Account.TradingAPIToken)
	if t := sess.Account.AccountToken(); t != "" {
		header.Set("Trading-Account-Token", t)
	}
	return c.send(ctx, endpoint, method, u, payload, header, sess.value(mode))
}

func (c *Client) send(ctx context.Context, name, method, rawURL string, payload []byte, header http.Header, coAuthValue string) (int, []byte, error) {
	for attempt := 0; ; attempt++ {
		if err := c.limiter.wait(ctx); err != nil {
			return 0, nil, err
		}
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range header {
			req.Header[k] = v
		}
		for _, ck := range c.jar.Cookies(req.URL) {
			if coAuthValue != "" && ck.Name == coAuth {
				continue
			}
			req.AddCookie(ck)
		}
		if coAuthValue != "" {
			req.AddCookie(&http.Cookie{Name: coAuth, Value: coAuthValue})
		}

		start := time.Now()
		resp, err := c.http.Do(req)
		var data []byte
		status := 0
		var retryAfter time.Duration
		if err == nil {
			data, err = io.ReadAll(resp.Body)
			resp.Body.Close()
			status = resp.StatusCode
			c.jar.SetCookies(req.URL, resp.Cookies())
			retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
		}
		c.record(Exchange{
			Name: name, Method: method, Path: req.URL.RequestURI(), RequestBody: payload,
			Status: status, ResponseBody: data, Duration: time.Since(start), Err: err,
		})
		if err != nil {
			if ctx.Err() != nil || attempt >= maxRetries {
				return 0, nil, fmt.Errorf("matchtrader %s: %w", name, err)
			}
			if err := c.sleep(ctx, c.backoff<<attempt); err != nil {
				return 0, nil, err
			}
			continue
		}
		if isChallenge(resp, data) {
			return status, data, ErrCloudflareChallenge
		}
		if (status == http.StatusTooManyRequests || status >= 500) && attempt < maxRetries {
			wait := retryAfter
			if wait <= 0 {
				wait = c.backoff << attempt
			}
			if err := c.sleep(ctx, wait); err != nil {
				return 0, nil, err
			}
			continue
		}
		return status, data, nil
	}
}

func (c *Client) sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func isChallenge(resp *http.Response, body []byte) bool {
	if resp.Header.Get("Cf-Mitigated") == "challenge" {
		return true
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "html") && !bytes.HasPrefix(bytes.TrimSpace(body), []byte("<")) {
		return false
	}
	s := string(body)
	return strings.Contains(s, "Just a moment") || strings.Contains(s, "cf-chl") || strings.Contains(s, "challenge-platform")
}

func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return time.Until(t)
	}
	return 0
}

type bucket struct {
	mu       sync.Mutex
	tokens   float64
	capacity float64
	rate     float64
	last     time.Time
}

func newBucket(perPeriod int, period time.Duration, burst int) *bucket {
	return &bucket{tokens: float64(burst), capacity: float64(burst), rate: float64(perPeriod) / period.Seconds(), last: time.Now()}
}

func (b *bucket) wait(ctx context.Context) error {
	for {
		b.mu.Lock()
		now := time.Now()
		b.tokens += now.Sub(b.last).Seconds() * b.rate
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}
		b.last = now
		if b.tokens >= 1 {
			b.tokens--
			b.mu.Unlock()
			return nil
		}
		need := time.Duration((1 - b.tokens) / b.rate * float64(time.Second))
		b.mu.Unlock()
		t := time.NewTimer(need)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

func (c *Client) Instruments(ctx context.Context) (json.RawMessage, error) {
	c.instrumentsMu.Lock()
	defer c.instrumentsMu.Unlock()
	if c.instruments != nil {
		return c.instruments, nil
	}
	var raw json.RawMessage
	if err := c.Do(ctx, http.MethodGet, "effective-instruments", nil, nil, &raw); err != nil {
		return nil, err
	}
	c.instruments = raw
	return raw, nil
}
