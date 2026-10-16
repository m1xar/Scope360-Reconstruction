package models

import "github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"

type Balance struct {
	Balance    fx.Number `json:"balance"`
	Equity     fx.Number `json:"equity"`
	Credit     fx.Number `json:"credit"`
	Margin     fx.Number `json:"margin"`
	FreeMargin fx.Number `json:"freeMargin"`
	Currency   string    `json:"currency"`
}
