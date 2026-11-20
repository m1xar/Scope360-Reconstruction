package ctrader

import (
	"errors"
	"fmt"

	"github.com/go-resty/resty/v2"
)

var tokenURL = "https://openapi.ctrader.com/apps/token"

// ErrTokenRefreshRejected marks a refresh cTrader refused (no refresh token, an OAuth
// error, a 4xx or a response without an access token). The stored grant is no longer
// usable and the user has to authorize the account again. Network failures and 5xx
// responses are not wrapped with it.
var ErrTokenRefreshRejected = errors.New("ctrader token refresh rejected")

type TokenSet struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	TokenType    string
}

type tokenResponse struct {
	AccessToken   string `json:"accessToken"`
	RefreshToken  string `json:"refreshToken"`
	ExpiresIn     int64  `json:"expiresIn"`
	TokenType     string `json:"tokenType"`
	AccessToken2  string `json:"access_token"`
	RefreshToken2 string `json:"refresh_token"`
	ExpiresIn2    int64  `json:"expires_in"`
	TokenType2    string `json:"token_type"`
	Error         string `json:"error"`
	ErrorDescr    string `json:"error_description"`
	ErrorCode     string `json:"errorCode"`
	Description   string `json:"description"`
}

func (c *Client) RefreshToken() (TokenSet, error) {
	if c.creds.RefreshToken == "" {
		return TokenSet{}, fmt.Errorf("%w: ctrader refresh token is empty", ErrTokenRefreshRejected)
	}
	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = resty.New()
	}
	var out tokenResponse
	resp, err := httpClient.R().
		SetResult(&out).
		SetQueryParam("grant_type", "refresh_token").
		SetQueryParam("refresh_token", c.creds.RefreshToken).
		SetQueryParam("client_id", c.creds.ClientID).
		SetQueryParam("client_secret", c.creds.ClientSecret).
		Get(tokenURL)
	if err != nil {
		return TokenSet{}, err
	}
	if out.Error == "" && out.ErrorCode != "" {
		out.Error, out.ErrorDescr = out.ErrorCode, out.Description
	}
	if resp.IsError() || out.Error != "" {
		var cause error = ErrTokenRefreshRejected
		if out.Error == "" && resp.StatusCode() >= 500 {
			cause = errors.New("ctrader token endpoint unavailable")
		}
		if out.ErrorDescr != "" {
			return TokenSet{}, fmt.Errorf("%w: ctrader token refresh failed: %s: %s", cause, out.Error, out.ErrorDescr)
		}
		return TokenSet{}, fmt.Errorf("%w: ctrader token refresh failed: http %d", cause, resp.StatusCode())
	}
	set := TokenSet{
		AccessToken:  firstNonEmpty(out.AccessToken, out.AccessToken2),
		RefreshToken: firstNonEmpty(out.RefreshToken, out.RefreshToken2),
		ExpiresIn:    firstNonZero(out.ExpiresIn, out.ExpiresIn2),
		TokenType:    firstNonEmpty(out.TokenType, out.TokenType2),
	}
	if set.AccessToken == "" {
		return TokenSet{}, fmt.Errorf("%w: ctrader token refresh returned empty access token", ErrTokenRefreshRejected)
	}
	if set.RefreshToken == "" {
		set.RefreshToken = c.creds.RefreshToken
	}
	c.creds.AccessToken = set.AccessToken
	c.creds.RefreshToken = set.RefreshToken
	if c.onTokenRefresh != nil {
		c.onTokenRefresh(set)
	}
	return set, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
