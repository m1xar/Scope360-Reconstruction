package models

import (
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

type OpenPosition struct {
	ID             string    `json:"id"`
	Symbol         string    `json:"symbol"`
	Alias          string    `json:"alias"`
	Volume         fx.Number `json:"volume"`
	Side           string    `json:"side"`
	OpenTime       Time      `json:"openTime"`
	OpenTimeMillis fx.Number `json:"openTimeMillis"`
	OpenPrice      fx.Number `json:"openPrice"`
	StopLoss       fx.Number `json:"stopLoss"`
	TakeProfit     fx.Number `json:"takeProfit"`
	Swap           fx.Number `json:"swap"`
	Profit         fx.Number `json:"profit"`
	NetProfit      fx.Number `json:"netProfit"`
	Commission     fx.Number `json:"commission"`
	CurrentPrice   fx.Number `json:"currentPrice"`
	Margin         fx.Number `json:"margin"`
}

func (c ClosedPosition) Reason() string {
	return strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(c.CloseReason)), "CLOSE_REASON_")
}

func (p OpenPosition) OpenedAt() time.Time { return Pick(p.OpenTimeMillis.Float(), p.OpenTime) }

type ClosedPosition struct {
	ID              string     `json:"id"`
	UID             string     `json:"uid"`
	ClosingOrderID  string     `json:"closingOrderID"`
	Symbol          string     `json:"symbol"`
	Alias           string     `json:"alias"`
	Side            string     `json:"side"`
	Volume          fx.Number  `json:"volume"`
	OpenTime        Time       `json:"openTime"`
	OpenTimeMillis  fx.Number  `json:"openTimeMillis"`
	OpenPrice       fx.Number  `json:"openPrice"`
	Time            Time       `json:"time"`
	TimeMillis      fx.Number  `json:"timeMillis"`
	CloseTimeMillis fx.Number  `json:"closeTimeMillis"`
	ClosePrice      fx.Number  `json:"closePrice"`
	StopLoss        fx.Number  `json:"stopLoss"`
	TakeProfit      fx.Number  `json:"takeProfit"`
	Commission      fx.Number  `json:"commission"`
	Swap            fx.Number  `json:"swap"`
	Profit          fx.Number  `json:"profit"`
	NetProfit       *fx.Number `json:"netProfit"`
	CloseReason     string     `json:"closeReason"`
}

func (c ClosedPosition) OpenedAt() time.Time { return Pick(c.OpenTimeMillis.Float(), c.OpenTime) }

func (c ClosedPosition) ClosedAt() time.Time {
	if c.TimeMillis > 0 {
		return FromEpoch(c.TimeMillis.Float())
	}
	return Pick(c.CloseTimeMillis.Float(), c.Time)
}

func (c ClosedPosition) Key() string {
	if c.UID != "" {
		return c.UID
	}
	return c.ID + "|" + c.ClosingOrderID + "|" + c.ClosedAt().Format(time.RFC3339Nano)
}

func (c ClosedPosition) Net() float64 {
	if c.NetProfit != nil {
		return c.NetProfit.Float()
	}
	return c.Profit.Float() + c.Commission.Float() + c.Swap.Float()
}
