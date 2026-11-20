package ctrader

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"

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

// RefreshToken exchanges the refresh token for a new pair. cTrader revokes the old
// pair on success, so refreshes of the same grant are serialized per process and a
// repeat with an already rotated refresh token gets the pair that replaced it instead
// of a second (failing) exchange: parallel clients of one account share one rotation.
func (c *Client) RefreshToken() (TokenSet, error) {
	if c.creds.RefreshToken == "" {
		return TokenSet{}, fmt.Errorf("%w: ctrader refresh token is empty", ErrTokenRefreshRejected)
	}
	key := sha256.Sum256([]byte(c.creds.ClientID + "\x00" + c.creds.RefreshToken))
	lock := refreshLocks.lock(key)
	defer lock.Unlock()
	set, ok := refreshLocks.rotated(key)
	if !ok {
		var err error
		if set, err = c.requestRefresh(); err != nil {
			return TokenSet{}, err
		}
		refreshLocks.remember(key, set)
	}
	c.creds.AccessToken = set.AccessToken
	c.creds.RefreshToken = set.RefreshToken
	if c.onTokenRefresh != nil {
		c.onTokenRefresh(set)
	}
	return set, nil
}

func (c *Client) requestRefresh() (TokenSet, error) {
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
	return set, nil
}

const rotatedTokenTTL = time.Hour

// refreshRegistry holds one lock per grant (client id + refresh token, hashed) and the
// pair each refresh produced, for rotatedTokenTTL.
type refreshRegistry struct {
	mu    sync.Mutex
	locks map[[32]byte]*sync.Mutex
	sets  map[[32]byte]rotatedSet
}

type rotatedSet struct {
	set TokenSet
	at  time.Time
}

var refreshLocks = &refreshRegistry{locks: map[[32]byte]*sync.Mutex{}, sets: map[[32]byte]rotatedSet{}}

func (r *refreshRegistry) lock(key [32]byte) *sync.Mutex {
	r.mu.Lock()
	now := time.Now()
	for k, item := range r.sets {
		if now.Sub(item.at) > rotatedTokenTTL {
			delete(r.sets, k)
			delete(r.locks, k)
		}
	}
	l, ok := r.locks[key]
	if !ok {
		l = &sync.Mutex{}
		r.locks[key] = l
	}
	r.mu.Unlock()
	l.Lock()
	return l
}

func (r *refreshRegistry) rotated(key [32]byte) (TokenSet, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.sets[key]
	if !ok || time.Since(item.at) > rotatedTokenTTL {
		return TokenSet{}, false
	}
	return item.set, true
}

func (r *refreshRegistry) remember(key [32]byte, set TokenSet) {
	r.mu.Lock()
	r.sets[key] = rotatedSet{set: set, at: time.Now()}
	r.mu.Unlock()
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
