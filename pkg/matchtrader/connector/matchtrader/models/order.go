package models

import (
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

type ActiveOrder struct {
	ID                 string    `json:"id"`
	Symbol             string    `json:"symbol"`
	Alias              string    `json:"alias"`
	Volume             fx.Number `json:"volume"`
	Type               string    `json:"type"`
	Side               string    `json:"side"`
	CreationTime       Time      `json:"creationTime"`
	CreationTimeIso    Time      `json:"creationTimeIso"`
	CreationTimeMillis fx.Number `json:"creationTimeMillis"`
	ActivationPrice    fx.Number `json:"activationPrice"`
	StopLoss           fx.Number `json:"stopLoss"`
	TakeProfit         fx.Number `json:"takeProfit"`
}

func (o ActiveOrder) CreatedAt() time.Time {
	if t := Pick(o.CreationTimeMillis.Float(), o.CreationTime); !t.IsZero() {
		return t
	}
	return o.CreationTimeIso.Time
}
