package builders

import (
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
)

const spotDustUSD = 0.10

func BuildSpotPositions(legs []helpers.SpotLeg, holdings []helpers.SpotHolding, windowStart time.Time, priceAt helpers.PriceAt) ([]domain.Position, []domain.OpenPosition) {
	byAsset := map[string][]helpers.SpotLeg{}
	for _, leg := range legs {
		if helpers.IsStableAsset(leg.Asset) {
			continue
		}
		byAsset[leg.Asset] = append(byAsset[leg.Asset], leg)
	}
	held := map[string]helpers.SpotHolding{}
	for _, h := range holdings {
		if helpers.IsStableAsset(h.Asset) {
			continue
		}
		held[h.Asset] = h
		if _, ok := byAsset[h.Asset]; !ok && h.Qty*h.PriceUSD >= spotDustUSD {
			byAsset[h.Asset] = nil
		}
	}

	assets := make([]string, 0, len(byAsset))
	for asset := range byAsset {
		assets = append(assets, asset)
	}
	sort.Strings(assets)

	var closed []domain.Position
	var open []domain.OpenPosition
	for _, asset := range assets {
		c, o := buildSpotAsset(asset, byAsset[asset], held[asset], windowStart, priceAt)
		closed = append(closed, c...)
		if o != nil {
			open = append(open, *o)
		}
	}
	sort.SliceStable(closed, func(i, j int) bool {
		return closed[i].ClosedAt.Before(*closed[j].ClosedAt)
	})
	return closed, open
}

type spotInventory struct {
	qty, cost, fee float64
}

func (inv *spotInventory) avg() float64 {
	if inv.qty <= 0 {
		return 0
	}
	return inv.cost / inv.qty
}

func buildSpotAsset(asset string, legs []helpers.SpotLeg, holding helpers.SpotHolding, windowStart time.Time, priceAt helpers.PriceAt) ([]domain.Position, *domain.OpenPosition) {
	sort.SliceStable(legs, func(i, j int) bool { return legs[i].Ts < legs[j].Ts })

	var buyQty, buyCost, sellQty float64
	for _, leg := range legs {
		if leg.Side == "buy" {
			buyQty += leg.Qty
			buyCost += leg.Qty * leg.PxUSD
		} else {
			sellQty += leg.Qty
		}
	}

	balance := holding.Qty
	eps := math.Max(balance, buyQty) * 1e-9

	var inv spotInventory
	var lotStart time.Time
	var lotOrders []domain.Order
	var closed []domain.Position
	synthetic := false

	syntheticPrice := func(qty float64) float64 {
		px := holding.AvgPx
		if px > 0 && buyQty > 0 {
			if cand := (px*(qty+buyQty) - buyCost) / qty; cand > 0 {
				px = cand
			}
		}
		if px <= 0 && priceAt != nil {
			if v, ok := priceAt(asset, windowStart.UnixMilli()); ok {
				px = v
			}
		}
		if px <= 0 && len(legs) > 0 {
			px = legs[0].PxUSD
		}
		return px
	}
	addSynthetic := func(qty float64) {
		px := syntheticPrice(qty)
		inv.qty += qty
		inv.cost += qty * px
		if !synthetic {
			lotStart = windowStart
			synthetic = true
		}
		entry := syntheticSpotOrder("BUY", qty, px, windowStart, 0)
		entry.Status = helpers.OrderStatusSynthetic
		lotOrders = append([]domain.Order{entry}, lotOrders...)
	}

	if missing := balance - buyQty + sellQty; missing > eps {
		addSynthetic(missing)
	}

	for _, leg := range legs {
		if leg.Side == "buy" {
			if inv.qty <= eps {
				lotStart = time.UnixMilli(leg.Ts).UTC()
				lotOrders = nil
				synthetic = false
			}
			inv.qty += leg.Qty
			inv.cost += leg.Qty * leg.PxUSD
			inv.fee += helpers.SpotFeeUSD(leg)
			lotOrders = append(lotOrders, spotOrderFromLeg(leg, 0))
			continue
		}

		if shortfall := leg.Qty - inv.qty; shortfall > eps {
			addSynthetic(shortfall)
		}
		avg := inv.avg()
		feeShare := 0.0
		if inv.qty > 0 {
			feeShare = inv.fee * leg.Qty / inv.qty
		}
		pos := spotClosedPosition(asset, leg, avg, helpers.SpotFeeUSD(leg)+feeShare, lotStart)
		if synthetic {
			pos.Orders[0].Status = helpers.OrderStatusSynthetic
		}
		closed = append(closed, pos)
		inv.qty -= leg.Qty
		inv.cost -= avg * leg.Qty
		inv.fee -= feeShare
		if inv.qty <= eps {
			inv = spotInventory{}
		}
		lotOrders = append(lotOrders, spotOrderFromLeg(leg, pos.NetPnl))
	}

	if inv.qty-balance > eps {
		avg := inv.avg()
		inv.fee *= balance / inv.qty
		inv.qty = balance
		inv.cost = avg * balance
	}

	if balance*holding.PriceUSD < spotDustUSD {
		return closed, nil
	}

	entry := inv.avg()
	if entry <= 0 {
		entry = holding.AvgPx
	}
	id, _ := uuid.NewV7()
	for i := range lotOrders {
		lotOrders[i].PositionID = id
	}
	quote := spotQuote(legs)
	return closed, &domain.OpenPosition{
		ID:           id,
		Pair:         asset + quote,
		Symbol:       asset + "-" + quote,
		Amount:       helpers.Round8(balance),
		Multiplier:   1,
		Side:         "LONG",
		EntryPrice:   helpers.Round8(entry),
		CurrentPrice: helpers.Round8(holding.PriceUSD),
		OpenTime:     lotStart,
		Orders:       lotOrders,
		IsSpot:       true,
	}
}

func spotClosedPosition(asset string, leg helpers.SpotLeg, entry, fee float64, openedAt time.Time) domain.Position {
	id, _ := uuid.NewV7()
	qty := leg.Qty
	exit := leg.PxUSD
	pnl := (exit - entry) * qty
	net := pnl - fee
	closedAt := time.UnixMilli(leg.Ts).UTC()

	status := "lose"
	if net > 0 {
		status = "win"
	}

	orders := []domain.Order{syntheticSpotOrder("BUY", qty, entry, openedAt, 0), spotOrderFromLeg(leg, net)}
	for i := range orders {
		orders[i].PositionID = id
	}

	return domain.Position{
		ID:         id,
		Side:       "LONG",
		Pair:       asset + leg.Quote,
		Symbol:     leg.InstId,
		Amount:     helpers.Round8(qty),
		EntryPrice: helpers.Round8(entry),
		ExitPrice:  helpers.Round8(exit),
		Pnl:        helpers.Round8(pnl),
		NetPnl:     helpers.Round8(net),
		Commission: helpers.Round8(fee),
		Multiplier: 1,
		Closed:     true,
		Status:     &status,
		CreatedAt:  openedAt,
		ClosedAt:   &closedAt,
		Orders:     orders,
		IsSpot:     true,
	}
}

func spotOrderFromLeg(leg helpers.SpotLeg, profit float64) domain.Order {
	id, _ := uuid.NewV7()
	side := helpers.OrderSideFromOKX(leg.Side)
	qty := helpers.Round8(leg.Qty)
	px := helpers.Round8(leg.PxUSD)
	doneAt := time.UnixMilli(leg.Ts).UTC()
	status := "FILLED"
	if leg.Transfer {
		status = helpers.OrderStatusTransfer
	}
	return domain.Order{
		ID:              id,
		ExchangeOrderID: leg.OrdId,
		Type:            helpers.OrderTypeFromOKX(leg.OrdType),
		Status:          status,
		Side:            side,
		Amount:          qty,
		AmountFilled:    qty,
		AveragePrice:    px,
		OriginalPrice:   px,
		UpdatedAt:       doneAt,
		Trade: domain.Trade{
			OrderID:    id,
			Side:       side,
			Price:      px,
			Amount:     qty,
			Commission: helpers.Round8(helpers.SpotFeeUSD(leg)),
			Profit:     helpers.Round8(profit),
			DoneAt:     doneAt,
		},
	}
}

func syntheticSpotOrder(side string, qty, px float64, at time.Time, profit float64) domain.Order {
	id, _ := uuid.NewV7()
	return domain.Order{
		ID:            id,
		Type:          "MARKET",
		Status:        "FILLED",
		Side:          side,
		Amount:        helpers.Round8(qty),
		AmountFilled:  helpers.Round8(qty),
		AveragePrice:  helpers.Round8(px),
		OriginalPrice: helpers.Round8(px),
		UpdatedAt:     at,
		Trade: domain.Trade{
			OrderID: id,
			Side:    side,
			Price:   helpers.Round8(px),
			Amount:  helpers.Round8(qty),
			Profit:  helpers.Round8(profit),
			DoneAt:  at,
		},
	}
}

func spotQuote(legs []helpers.SpotLeg) string {
	for i := len(legs) - 1; i >= 0; i-- {
		if legs[i].Quote != "" {
			return legs[i].Quote
		}
	}
	return "USDT"
}
