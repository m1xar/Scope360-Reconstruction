package matchtrader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/matchtrader/connector/matchtrader/models"
)

type Session struct {
	Token      string
	SystemUUID string
	Account    models.TradingAccount
	Created    time.Time
}

func (s *Session) value(mode coAuthMode) string {
	if mode == coAuthAccount {
		return s.Account.AccountToken()
	}
	return s.Token
}

func (c *Client) Session() *Session {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.session
}

func (c *Client) EnsureSession(ctx context.Context) (*Session, error) {
	if s := c.Session(); s != nil {
		return s, nil
	}
	c.authMu.Lock()
	defer c.authMu.Unlock()
	if s := c.Session(); s != nil {
		return s, nil
	}
	return c.login(ctx)
}

func (c *Client) relogin(ctx context.Context, stale *Session) (*Session, error) {
	c.authMu.Lock()
	defer c.authMu.Unlock()
	if s := c.Session(); s != nil && s != stale {
		return s, nil
	}
	return c.login(ctx)
}

func (c *Client) refresh(ctx context.Context, stale *Session) (*Session, error) {
	c.authMu.Lock()
	defer c.authMu.Unlock()
	if s := c.Session(); s != nil && s != stale {
		return s, nil
	}
	var lastErr error
	for _, path := range []string{"/manager/refresh-token", "/mtr-core-edge/refresh-token"} {
		status, data, err := c.send(ctx, "refresh-token", http.MethodPost, c.base+path, nil, nil, "")
		if err != nil {
			return nil, err
		}
		if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
			lastErr = &HTTPError{Name: "refresh-token", Status: status, Body: string(data)}
			continue
		}
		if status < 200 || status >= 300 {
			return nil, &HTTPError{Name: "refresh-token", Status: status, Body: string(data)}
		}
		var body struct {
			Token string `json:"token"`
		}
		_ = json.Unmarshal(data, &body)
		token := body.Token
		if token == "" {
			token = c.cookie(coAuth)
		}
		if token == "" || token == stale.Token {
			return nil, errors.New("matchtrader refresh-token: no new token")
		}
		next := *stale
		next.Token = token
		c.mu.Lock()
		c.session = &next
		c.mu.Unlock()
		return &next, nil
	}
	return nil, lastErr
}

func (c *Client) login(ctx context.Context) (*Session, error) {
	if c.base == "" || c.cfg.Email == "" || c.cfg.Password == "" || strings.TrimSpace(c.cfg.AccountID) == "" {
		return nil, errors.New("matchtrader: BaseURL, Email, Password and AccountID are required")
	}
	brokerID := c.platformPartnerID(ctx)

	attempts := []struct {
		path, userField string
	}{
		{"/manager/mtr-login", "email"},
		{"/mtr-core-edge/login", "email"},
		{"/mtr-core-edge/login", "login"},
	}
	var lastErr error
	for _, a := range attempts {
		body, _ := json.Marshal(map[string]string{a.userField: c.cfg.Email, "password": c.cfg.Password, "brokerId": brokerID})
		status, data, err := c.send(ctx, "login", http.MethodPost, c.base+a.path, body, nil, "")
		if err != nil {
			return nil, err
		}
		if status < 200 || status >= 300 {
			lastErr = &HTTPError{Name: "login", Status: status, Body: string(data)}
			if status == http.StatusNotFound || status == http.StatusMethodNotAllowed || status == http.StatusBadRequest {
				continue
			}
			return nil, lastErr
		}
		var res models.LoginResponse
		if err := json.Unmarshal(data, &res); err != nil {
			lastErr = fmt.Errorf("matchtrader login: decode: %w", err)
			continue
		}
		if res.Token == "" {
			res.Token = c.cookie(coAuth)
		}
		if res.Token == "" && len(res.AllAccounts()) == 0 {
			lastErr = errors.New("matchtrader login: no token in response")
			continue
		}
		sess, err := c.newSession(res, brokerID)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.session = sess
		c.mu.Unlock()
		return sess, nil
	}
	return nil, lastErr
}

func (c *Client) platformPartnerID(ctx context.Context) string {
	status, data, err := c.send(ctx, "platform-details", http.MethodGet, c.base+"/manager/platform-details", nil, nil, "")
	if err != nil || status != http.StatusOK {
		return "0"
	}
	var details models.PlatformDetails
	if json.Unmarshal(data, &details) != nil || details.PartnerID == "" {
		return "0"
	}
	return details.PartnerID.String()
}

func (c *Client) newSession(res models.LoginResponse, brokerID string) (*Session, error) {
	account, ok := pickAccount(res.AllAccounts(), res.SelectedTradingAccount, strings.TrimSpace(c.cfg.AccountID))
	if !ok {
		return nil, fmt.Errorf("matchtrader: trading account %q not found", c.cfg.AccountID)
	}
	return &Session{
		Token: res.Token,

		SystemUUID: account.Offer.System.UUID,
		Account:    account,
		Created:    account.Created.Time,
	}, nil
}

func pickAccount(accounts []models.TradingAccount, selected *models.TradingAccount, id string) (models.TradingAccount, bool) {
	if selected != nil {
		accounts = append(accounts, *selected)
	}
	for _, a := range accounts {
		if (a.TradingAccountID.String() == id || a.UUID == id) && a.Offer.System.UUID != "" {
			return a, true
		}
	}
	return models.TradingAccount{}, false
}

func (c *Client) cookie(name string) string {
	u, err := url.Parse(c.base + "/")
	if err != nil {
		return ""
	}
	for _, ck := range c.jar.Cookies(u) {
		if ck.Name == name {
			return ck.Value
		}
	}
	return ""
}
