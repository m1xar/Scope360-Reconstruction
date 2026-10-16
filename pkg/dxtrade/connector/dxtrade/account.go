package dxtrade

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
)

func (c *Client) Username() string {
	if strings.Contains(c.cfg.Username, "@") {
		return c.cfg.Username
	}
	domain := c.Domain()
	if domain == "" {
		domain = c.cfg.Domain
	}
	if domain == "" {
		domain = DefaultDomain
	}
	return c.cfg.Username + "@" + domain
}

func (c *Client) Account(ctx context.Context) (*models.AccountDetails, error) {
	c.accountMu.Lock()
	defer c.accountMu.Unlock()
	if c.account != nil {
		return c.account, nil
	}
	if _, err := c.ensureSession(ctx); err != nil {
		return nil, err
	}
	var list models.UserDetailsList
	if err := c.Do(ctx, "user", http.MethodGet, "/users/"+EscapeSegment(c.Username()), nil, nil, &list); err != nil {
		return nil, err
	}
	var accounts []models.AccountDetails
	for _, u := range list.UserDetails {
		accounts = append(accounts, u.Accounts...)
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("dxtrade: user %s has no accounts", c.Username())
	}
	want := strings.TrimSpace(c.cfg.Account)
	for i := range accounts {
		code := accounts[i].Account
		if code == want || strings.HasSuffix(code, ":"+want) {
			c.account = &accounts[i]
			return c.account, nil
		}
	}
	return nil, fmt.Errorf("dxtrade: account %q not found", want)
}

func (c *Client) AccountPath(ctx context.Context, suffix string) (string, error) {
	account, err := c.Account(ctx)
	if err != nil {
		return "", err
	}
	return "/accounts/" + EscapeSegment(account.Account) + suffix, nil
}

func (c *Client) IsDemo(account *models.AccountDetails) bool {
	parts := []string{c.host}
	if account != nil {
		parts = append(parts, account.Account, account.AccountStatus, account.PricingStream)
	}
	for _, p := range parts {
		if strings.Contains(strings.ToLower(p), "demo") {
			return true
		}
	}
	return false
}
