package ctrader

import (
	"context"
	"fmt"

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
	if err := c.Connect(ctx, session.Endpoint()); err != nil {
		return nil, err
	}
	c.session = session
	if err := c.applicationAuth(ctx); err != nil {
		return nil, err
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
	return c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_APPLICATION_AUTH_REQ, req, &res)
}

func (c *Client) accountAuth(ctx context.Context, retry bool) error {
	if c.session == nil {
		return fmt.Errorf("ctrader session is not resolved")
	}
	accountID := c.session.CtidTraderAccountID
	accessToken := c.creds.AccessToken
	req := &pb.ProtoOAAccountAuthReq{CtidTraderAccountId: &accountID, AccessToken: &accessToken}
	var res pb.ProtoOAAccountAuthRes
	if retry {
		return c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_ACCOUNT_AUTH_REQ, req, &res)
	}
	return c.doOnce(ctx, pb.ProtoOAPayloadType_PROTO_OA_ACCOUNT_AUTH_REQ, req, &res)
}

func (c *Client) resolveAccount(ctx context.Context) (*pb.ProtoOACtidTraderAccount, error) {
	accessToken := c.creds.AccessToken
	req := &pb.ProtoOAGetAccountListByAccessTokenReq{AccessToken: &accessToken}
	var res pb.ProtoOAGetAccountListByAccessTokenRes
	if err := c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_GET_ACCOUNTS_BY_ACCESS_TOKEN_REQ, req, &res); err != nil {
		return nil, err
	}
	for _, account := range res.GetCtidTraderAccount() {
		if account.GetTraderLogin() == c.creds.AccountLogin {
			return account, nil
		}
	}
	return nil, fmt.Errorf("ctrader account login %d not found in access token account list", c.creds.AccountLogin)
}
