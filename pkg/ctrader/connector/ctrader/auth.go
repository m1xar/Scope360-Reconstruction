package ctrader

import (
	"context"
	"fmt"

	gproto "google.golang.org/protobuf/proto"

	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

func (c *Client) AuthSession(ctx context.Context) (*Session, error) {
	if err := c.Connect(ctx, LiveEndpoint); err != nil {
		return nil, err
	}
	if err := c.applicationAuth(ctx); err != nil {
		return nil, err
	}
	account, err := c.resolveAccount(ctx)
	if err != nil {
		return nil, err
	}
	session := &Session{
		CtidTraderAccountID: int64(account.GetCtidTraderAccountId()),
		AccountLogin:        account.GetTraderLogin(),
		IsLive:              account.GetIsLive(),
		BrokerTitleShort:    account.GetBrokerTitleShort(),
	}
	// A live account stays on the endpoint the application is already authorized on:
	// reconnecting returns the same open stream, and a second ApplicationAuth on it is
	// answered with ALREADY_LOGGED_IN. Only a changed endpoint (demo) needs a new
	// connection and its own application auth.
	if endpoint := session.Endpoint(); endpoint != c.endpoint {
		if err := c.Connect(ctx, endpoint); err != nil {
			return nil, err
		}
		c.session = session
		if err := c.applicationAuth(ctx); err != nil {
			return nil, err
		}
	} else {
		c.session = session
	}
	if err := c.accountAuth(ctx, true); err != nil {
		return nil, err
	}
	trader, err := c.FetchTrader(ctx)
	if err == nil && trader != nil {
		c.session.MoneyDigits = trader.GetMoneyDigits()
		c.session.LeverageInCents = trader.GetLeverageInCents()
		c.session.TotalMarginCalculationType = trader.GetTotalMarginCalculationType().String()
		c.session.MaxLeverage = trader.GetMaxLeverage()
		c.session.IsLimitedRisk = trader.GetIsLimitedRisk()
	}
	return c.session, nil
}

func (c *Client) EnsureSession(ctx context.Context) (*Session, error) {
	if c.session != nil {
		return c.session, nil
	}
	return c.AuthSession(ctx)
}

func (c *Client) applicationAuth(ctx context.Context) error {
	clientID := c.creds.ClientID
	clientSecret := c.creds.ClientSecret
	req := &pb.ProtoOAApplicationAuthReq{ClientId: &clientID, ClientSecret: &clientSecret}
	var res pb.ProtoOAApplicationAuthRes
	return ignoreAlreadyAuthorized(c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_APPLICATION_AUTH_REQ, req, &res))
}

func (c *Client) accountAuth(ctx context.Context, retry bool) error {
	if c.session == nil {
		return fmt.Errorf("ctrader session is not resolved")
	}
	accountID := c.session.CtidTraderAccountID
	build := func(accessToken string) gproto.Message {
		return &pb.ProtoOAAccountAuthReq{CtidTraderAccountId: &accountID, AccessToken: &accessToken}
	}
	var res pb.ProtoOAAccountAuthRes
	if retry {
		return ignoreAlreadyAuthorized(c.doWithAccessToken(ctx, pb.ProtoOAPayloadType_PROTO_OA_ACCOUNT_AUTH_REQ, build, &res))
	}
	return ignoreAlreadyAuthorized(c.doOnce(ctx, pb.ProtoOAPayloadType_PROTO_OA_ACCOUNT_AUTH_REQ, build(c.creds.AccessToken), &res))
}

// doWithAccessToken sends a request that carries the access token itself. On an auth
// error it refreshes the token once and rebuilds the request with the new token: a
// retry of the original request would resend the token the refresh just revoked.
func (c *Client) doWithAccessToken(ctx context.Context, payloadType pb.ProtoOAPayloadType, build func(accessToken string) gproto.Message, res gproto.Message) error {
	err := c.doOnce(ctx, payloadType, build(c.creds.AccessToken), res)
	if !isAuthError(err) {
		return err
	}
	if _, refreshErr := c.RefreshToken(); refreshErr != nil {
		return fmt.Errorf("%w; token refresh failed: %w", err, refreshErr)
	}
	return c.doOnce(ctx, payloadType, build(c.creds.AccessToken), res)
}

// ignoreAlreadyAuthorized treats ALREADY_LOGGED_IN as success: the step it answers
// (application or account auth on this connection) has already been done.
func ignoreAlreadyAuthorized(err error) error {
	if IsAlreadyAuthorized(err) {
		return nil
	}
	return err
}

func (c *Client) resolveAccount(ctx context.Context) (*pb.ProtoOACtidTraderAccount, error) {
	build := func(accessToken string) gproto.Message {
		return &pb.ProtoOAGetAccountListByAccessTokenReq{AccessToken: &accessToken}
	}
	var res pb.ProtoOAGetAccountListByAccessTokenRes
	if err := c.doWithAccessToken(ctx, pb.ProtoOAPayloadType_PROTO_OA_GET_ACCOUNTS_BY_ACCESS_TOKEN_REQ, build, &res); err != nil {
		return nil, err
	}
	for _, account := range res.GetCtidTraderAccount() {
		if account.GetTraderLogin() == c.creds.AccountLogin {
			return account, nil
		}
	}
	return nil, fmt.Errorf("ctrader account login %d not found in access token account list", c.creds.AccountLogin)
}
