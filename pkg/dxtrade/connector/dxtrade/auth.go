package dxtrade

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
)

const DefaultDomain = "default"

var (
	vendorPattern = regexp.MustCompile(`x-dx-vendor'\]\s*=\s*['"]([^'"]+)`)
	isoDuration   = regexp.MustCompile(`^P(?:(\d+(?:\.\d+)?)D)?(?:T(?:(\d+(?:\.\d+)?)H)?(?:(\d+(?:\.\d+)?)M)?(?:(\d+(?:\.\d+)?)S)?)?$`)
)

func (c *Client) Login(ctx context.Context) error {
	c.loginMu.Lock()
	defer c.loginMu.Unlock()
	return c.loginLocked(ctx)
}

func (c *Client) loginLocked(ctx context.Context) error {
	if c.host == "" || c.cfg.Username == "" || c.cfg.Password == "" || strings.TrimSpace(c.cfg.Account) == "" {
		return errors.New("dxtrade: BaseURL, Username, Password and Account are required")
	}
	c.mu.Lock()
	known := c.domain
	c.mu.Unlock()

	domains := []string{known, c.cfg.Domain, DefaultDomain}
	if vendor := c.vendorDomain(ctx); vendor != "" {
		domains = append(domains, vendor)
	}
	tried := map[string]bool{}
	var lastErr error
	for _, domain := range domains {
		if domain == "" || tried[domain] {
			continue
		}
		tried[domain] = true
		err := c.loginDomain(ctx, domain)
		if err == nil {
			return nil
		}
		lastErr = err
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Status < 400 || apiErr.Status >= 500 {
			return err
		}
	}
	return fmt.Errorf("dxtrade login failed: %w", lastErr)
}

func (c *Client) loginDomain(ctx context.Context, domain string) error {
	if err := c.login.Wait(ctx); err != nil {
		return err
	}
	req := models.LoginRequest{Username: c.cfg.Username, Domain: domain, Password: c.cfg.Password}
	var res models.SessionToken
	if err := c.call(ctx, "login", http.MethodPost, c.host+apiPrefix+"/login", nil, req, &res, ""); err != nil {
		return err
	}
	if res.SessionToken == "" {
		return errors.New("dxtrade login: empty session token")
	}
	c.mu.Lock()
	c.token, c.domain = res.SessionToken, domain
	c.timeout = ParseTimeout(res.Timeout)
	c.lastUse = time.Now()
	c.mu.Unlock()
	return nil
}

func (c *Client) vendorDomain(ctx context.Context) string {
	if err := c.rate.Wait(ctx); err != nil {
		return ""
	}
	u, err := url.Parse(c.host + "/")
	if err != nil {
		return ""
	}
	status, _, page, err := c.send(ctx, "login_page", http.MethodGet, u, nil, "")
	if err != nil || status != http.StatusOK {
		return ""
	}
	if m := vendorPattern.FindSubmatch(page); m != nil {
		return string(m[1])
	}
	return ""
}

func (c *Client) relogin(ctx context.Context, stale string) (string, error) {
	c.loginMu.Lock()
	defer c.loginMu.Unlock()
	c.mu.Lock()
	current := c.token
	c.mu.Unlock()
	if current != "" && current != stale {
		return current, nil
	}
	if err := c.loginLocked(ctx); err != nil {
		return "", err
	}
	return c.Token(), nil
}

func (c *Client) ensureSession(ctx context.Context) (string, error) {
	c.mu.Lock()
	token, timeout, last := c.token, c.timeout, c.lastUse
	c.mu.Unlock()
	if token == "" {
		if err := c.Login(ctx); err != nil {
			return "", err
		}
		return c.Token(), nil
	}
	if timeout > 0 && time.Since(last) > timeout/2 {
		if err := c.Ping(ctx); err != nil {
			return c.relogin(ctx, token)
		}
		return c.Token(), nil
	}
	return token, nil
}

func (c *Client) Ping(ctx context.Context) error {
	token := c.Token()
	if token == "" {
		return errors.New("dxtrade: not logged in")
	}
	var res models.SessionToken
	if err := c.call(ctx, "ping", http.MethodPost, c.host+apiPrefix+"/ping", nil, nil, &res, token); err != nil {
		return err
	}
	c.mu.Lock()
	if res.SessionToken != "" {
		c.token = res.SessionToken
	}
	if t := ParseTimeout(res.Timeout); t > 0 {
		c.timeout = t
	}
	c.lastUse = time.Now()
	c.mu.Unlock()
	return nil
}

func (c *Client) Logout(ctx context.Context) error {
	token := c.Token()
	if token == "" {
		return nil
	}
	c.mu.Lock()
	c.token = ""
	c.mu.Unlock()
	return c.call(ctx, "logout", http.MethodPost, c.host+apiPrefix+"/logout", nil, nil, nil, token)
}

func (c *Client) Token() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token
}

func (c *Client) Domain() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.domain
}

func ParseTimeout(raw json.RawMessage) time.Duration {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return 0
	}
	switch raw[0] {
	case '"':
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return 0
		}
		return parseTimeoutString(s)
	case '{':
		var obj struct {
			Seconds float64 `json:"seconds"`
			Nano    float64 `json:"nano"`
		}
		if json.Unmarshal(raw, &obj) != nil {
			return 0
		}
		return time.Duration(obj.Seconds*float64(time.Second) + obj.Nano)
	}
	return parseTimeoutString(string(raw))
}

func parseTimeoutString(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if secs, err := strconv.ParseFloat(s, 64); err == nil {
		return time.Duration(secs * float64(time.Second))
	}
	if m := isoDuration.FindStringSubmatch(strings.ToUpper(s)); m != nil {
		units := []time.Duration{24 * time.Hour, time.Hour, time.Minute, time.Second}
		var total time.Duration
		for i, unit := range units {
			if m[i+1] != "" {
				v, _ := strconv.ParseFloat(m[i+1], 64)
				total += time.Duration(v * float64(unit))
			}
		}
		return total
	}
	if parts := strings.Split(s, ":"); len(parts) == 3 {
		h, e1 := strconv.Atoi(parts[0])
		m, e2 := strconv.Atoi(parts[1])
		sec, e3 := strconv.ParseFloat(parts[2], 64)
		if e1 == nil && e2 == nil && e3 == nil {
			return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec*float64(time.Second))
		}
	}
	if d, err := time.ParseDuration(strings.ToLower(s)); err == nil {
		return d
	}
	return 0
}
