package models

import (
	"bytes"
	"encoding/json"

	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

type PlatformDetails struct {
	PartnerID   Str    `json:"partnerId"`
	PlatformURL string `json:"platformUrl"`
	BrokerName  string `json:"brokerName"`
}

type LoginResponse struct {
	Token                  string           `json:"token"`
	TradingAccounts        []TradingAccount `json:"tradingAccounts"`
	Accounts               []TradingAccount `json:"accounts"`
	SelectedTradingAccount *TradingAccount  `json:"selectedTradingAccount"`
}

func (r LoginResponse) AllAccounts() []TradingAccount {
	if len(r.TradingAccounts) > 0 {
		return r.TradingAccounts
	}
	return r.Accounts
}

type TradingAccount struct {
	TradingAccountID    Str           `json:"tradingAccountId"`
	UUID                string        `json:"uuid"`
	Offer               Offer         `json:"offer"`
	TradingAPIToken     string        `json:"tradingApiToken"`
	TradingAccountToken *AccountToken `json:"tradingAccountToken"`
	Created             Time          `json:"created"`
	Leverage            fx.Number     `json:"leverage"`
}

func (a TradingAccount) AccountToken() string {
	if a.TradingAccountToken == nil {
		return ""
	}
	return a.TradingAccountToken.Token
}

type Offer struct {
	UUID           string    `json:"uuid"`
	PartnerID      Str       `json:"partnerId"`
	Created        Time      `json:"created"`
	Name           string    `json:"name"`
	Currency       string    `json:"currency"`
	InitialDeposit fx.Number `json:"initialDeposit"`
	Demo           bool      `json:"demo"`
	Leverage       fx.Number `json:"leverage"`
	System         System    `json:"system"`
}

type System struct {
	UUID             string `json:"uuid"`
	Name             string `json:"name"`
	Demo             bool   `json:"demo"`
	TradingAPIDomain string `json:"tradingApiDomain"`
}

type AccountToken struct {
	Token      string `json:"token"`
	Expiration Time   `json:"expiration"`
}

func (t *AccountToken) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		return json.Unmarshal(data, &t.Token)
	}
	type plain AccountToken
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*t = AccountToken(p)
	return nil
}
