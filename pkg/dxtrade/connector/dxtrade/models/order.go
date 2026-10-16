package models

import "github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"

const (
	EffectClose = "CLOSE"

	StatusAccepted  = "ACCEPTED"
	StatusWorking   = "WORKING"
	StatusCanceled  = "CANCELED"
	StatusCompleted = "COMPLETED"
	StatusExpired   = "EXPIRED"
	StatusRejected  = "REJECTED"
)

type OrderList struct {
	Orders                  []Order `json:"orders"`
	NextPageTransactionTime Time    `json:"nextPageTransactionTime"`
}

type Order struct {
	Account          string            `json:"account"`
	Version          fx.Number         `json:"version"`
	OrderID          ID                `json:"orderId"`
	OrderCode        string            `json:"orderCode"`
	ClientOrderID    string            `json:"clientOrderId"`
	ActionCode       string            `json:"actionCode"`
	LegCount         fx.Number         `json:"legCount"`
	Type             string            `json:"type"`
	Instrument       string            `json:"instrument"`
	Status           string            `json:"status"`
	FinalStatus      Bool              `json:"finalStatus"`
	Legs             []OrderLeg        `json:"legs"`
	Side             string            `json:"side"`
	TIF              string            `json:"tif"`
	PriceOffset      fx.Number         `json:"priceOffset"`
	PriceLink        string            `json:"priceLink"`
	MarginRate       fx.Number         `json:"marginRate"`
	IssueTime        Time              `json:"issueTime"`
	TransactionTime  Time              `json:"transactionTime"`
	Links            []OrderLink       `json:"links"`
	Executions       []Execution       `json:"executions"`
	CashTransactions []CashTransaction `json:"cashTransactions"`
}

type OrderLeg struct {
	Instrument        string    `json:"instrument"`
	PositionEffect    string    `json:"positionEffect"`
	PositionCode      ID        `json:"positionCode"`
	Side              string    `json:"side"`
	Price             fx.Number `json:"price"`
	LegRatio          fx.Number `json:"legRatio"`
	Quantity          fx.Number `json:"quantity"`
	FilledQuantity    fx.Number `json:"filledQuantity"`
	RemainingQuantity fx.Number `json:"remainingQuantity"`
	AveragePrice      fx.Number `json:"averagePrice"`
}

type OrderLink struct {
	LinkType            string `json:"linkType"`
	LinkedOrder         string `json:"linkedOrder"`
	LinkedClientOrderID string `json:"linkedClientOrderId"`
}

type Execution struct {
	Account           string    `json:"account"`
	ExecutionCode     ID        `json:"executionCode"`
	OrderCode         string    `json:"orderCode"`
	UpdateOrderID     ID        `json:"updateOrderId"`
	Version           fx.Number `json:"version"`
	ClientOrderID     string    `json:"clientOrderId"`
	ActionCode        string    `json:"actionCode"`
	Instrument        string    `json:"instrument"`
	Status            string    `json:"status"`
	FinalStatus       Bool      `json:"finalStatus"`
	FilledQuantity    fx.Number `json:"filledQuantity"`
	LastQuantity      fx.Number `json:"lastQuantity"`
	RemainingQuantity fx.Number `json:"remainingQuantity"`
	LastPrice         fx.Number `json:"lastPrice"`
	AveragePrice      fx.Number `json:"averagePrice"`
	TransactionTime   Time      `json:"transactionTime"`
	RejectReason      string    `json:"rejectReason"`
}

func (o Order) Leg() OrderLeg {
	if len(o.Legs) == 0 {
		return OrderLeg{Instrument: o.Instrument}
	}
	return o.Legs[0]
}

func (o Order) Symbol() string {
	if o.Instrument != "" {
		return o.Instrument
	}
	return o.Leg().Instrument
}

func (o Order) OrderSide() string {
	if o.Side != "" {
		return o.Side
	}
	return o.Leg().Side
}

func (o Order) Closing() bool { return o.Leg().PositionEffect == EffectClose }

func (o Order) PositionCode() string {
	if code := o.Leg().PositionCode.String(); code != "" {
		return code
	}
	if o.Closing() {
		return ""
	}
	return o.OrderID.String()
}

func (o Order) Key() string {
	if id := o.OrderID.String(); id != "" {
		return id
	}
	return o.OrderCode
}

func (o Order) Newer(than Order) bool {
	if o.Version != than.Version {
		return o.Version > than.Version
	}
	if !o.TransactionTime.Equal(than.TransactionTime.Time) {
		return o.TransactionTime.After(than.TransactionTime.Time)
	}
	return len(o.Executions)+len(o.CashTransactions) > len(than.Executions)+len(than.CashTransactions)
}
