package helpers

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/dxtrade/connector/dxtrade/models"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/fx"
)

const Epsilon = 1e-9

type Fill struct {
	OrderID       string
	ExecutionCode string
	PositionCode  string
	Symbol        string
	Side          string
	Closing       bool
	Price         float64
	Quantity      float64
	Time          time.Time
}

func Fills(o models.Order) []Fill {
	base := Fill{
		OrderID:      o.OrderID.String(),
		PositionCode: o.PositionCode(),
		Symbol:       o.Symbol(),
		Side:         strings.ToUpper(o.OrderSide()),
		Closing:      o.Closing(),
	}
	seen := make(map[string]bool)
	var out []Fill
	for _, e := range o.Executions {
		qty := math.Abs(e.LastQuantity.Float())
		if qty <= Epsilon || e.LastPrice.Float() <= 0 {
			continue
		}
		code := e.ExecutionCode.String()
		if code != "" {
			if seen[code] {
				continue
			}
			seen[code] = true
		}
		f := base
		f.ExecutionCode, f.Price, f.Quantity, f.Time = code, e.LastPrice.Float(), qty, e.TransactionTime.Time
		out = append(out, f)
	}
	if len(out) == 0 {
		leg := o.Leg()
		if qty := math.Abs(leg.FilledQuantity.Float()); qty > Epsilon && leg.AveragePrice.Float() > 0 {
			f := base
			f.Price, f.Quantity, f.Time = leg.AveragePrice.Float(), qty, o.TransactionTime.Time
			out = append(out, f)
		}
	}
	SortFills(out)
	return out
}

func SortFills(fills []Fill) {
	sort.SliceStable(fills, func(i, j int) bool { return fills[i].Time.Before(fills[j].Time) })
}

func Quantity(fills []Fill) float64 {
	var q float64
	for _, f := range fills {
		q += f.Quantity
	}
	return q
}

func VWAP(fills []Fill) float64 {
	pairs := make([]fx.Fill, 0, len(fills))
	for _, f := range fills {
		pairs = append(pairs, fx.Fill{Price: f.Price, Quantity: f.Quantity})
	}
	return fx.VWAP(pairs)
}

func (f Fill) Signed() float64 {
	if f.Side == fx.Sell {
		return -f.Quantity
	}
	return f.Quantity
}
