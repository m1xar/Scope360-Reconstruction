package models

type HyperliquidCandle struct {
	T  int64  `json:"T"`
	C  string `json:"c"`
	H  string `json:"h"`
	I  string `json:"i"`
	L  string `json:"l"`
	N  int    `json:"n"`
	O  string `json:"o"`
	S  string `json:"s"`
	T0 int64  `json:"t"`
	V  string `json:"v"`
}
