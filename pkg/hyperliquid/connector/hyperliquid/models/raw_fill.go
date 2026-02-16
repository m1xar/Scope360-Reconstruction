package models

type RawFill struct {
	Coin          string  `json:"coin"`
	Px            string  `json:"px"`
	Sz            string  `json:"sz"`
	Side          string  `json:"side"`
	Time          int64   `json:"time"`
	StartPosition string  `json:"startPosition"`
	Dir           string  `json:"dir"`
	ClosedPnl     string  `json:"closedPnl"`
	Hash          string  `json:"hash"`
	Oid           int64   `json:"oid"`
	Crossed       bool    `json:"crossed"`
	Fee           string  `json:"fee"`
	Tid           int64   `json:"tid"`
	FeeToken      string  `json:"feeToken"`
	TwapId        *string `json:"twapId"`
}
