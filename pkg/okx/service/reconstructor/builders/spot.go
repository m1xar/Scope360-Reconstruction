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

// BuildSpotPositions reconstructs one lot per asset: the lot opens with the
// first buy (or a synthetic basis when history starts mid-lot), absorbs every
// buy and sell as an execution and closes when the remaining quantity falls
// below the instrument's minimum order size. A closed lot is a closed position
// with volume-weighted entry/exit prices; the live lot is the open position.
func BuildSpotPositions(legs []helpers.SpotLeg, holdings []helpers.SpotHolding, windowStart time.Time, priceAt helpers.PriceAt, minSizes map[string]float64) ([]domain.Position, []domain.OpenPosition) {
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
		c, o := buildSpotAsset(asset, byAsset[asset], held[asset], windowStart, priceAt, minSizes[asset])
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
	qty, cost float64
}

func (inv *spotInventory) avg() float64 {
	if inv.qty <= 0 {
		return 0
	}
	return inv.cost / inv.qty
}

type spotLot struct {
	start     time.Time
	orders    []domain.Order
	buyQty    float64
	buyCost   float64
	buyFee    float64
	sellQty   float64
	proceeds  float64
	sellFee   float64
	realized  float64
	lastSell  time.Time
	quote     string
	instId    string
	synthetic bool
}

func buildSpotAsset(asset string, legs []helpers.SpotLeg, holding helpers.SpotHolding, windowStart time.Time, priceAt helpers.PriceAt, minSize float64) ([]domain.Position, *domain.OpenPosition) {
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
	var lot spotLot
	var closed []domain.Position

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
	addSynthetic := func(qty float64, at time.Time) {
		px := syntheticPrice(qty)
		inv.qty += qty
		inv.cost += qty * px
		lot.buyQty += qty
		lot.buyCost += qty * px
		lot.synthetic = true
		if lot.start.IsZero() {
			lot.start = at
		}
		entry := syntheticSpotOrder("BUY", qty, px, at, 0)
		entry.Status = helpers.OrderStatusSynthetic
		lot.orders = append(lot.orders, entry)
	}
	openLot := func(at time.Time) {
		if !lot.start.IsZero() {
			return
		}
		lot.start = at
		lot.buyQty += inv.qty
		lot.buyCost += inv.cost
	}
	dust := func(qty float64) bool {
		if qty <= eps {
			return true
		}
		if minSize > 0 && qty < minSize {
			return true
		}
		return holding.PriceUSD > 0 && qty*holding.PriceUSD < spotDustUSD
	}
	closeLot := func() {
		if lot.sellQty > 0 {
			closed = append(closed, spotClosedPosition(asset, lot))
		}
		if inv.qty <= eps {
			inv = spotInventory{}
		}
		lot = spotLot{}
	}

	if missing := balance - buyQty + sellQty; missing > eps {
		addSynthetic(missing, windowStart)
	}

	for _, leg := range legs {
		at := time.UnixMilli(leg.Ts).UTC()
		if leg.Side == "buy" {
			openLot(at)
			if leg.Quote != "" {
				lot.quote, lot.instId = leg.Quote, leg.InstId
			}
			fee := helpers.SpotFeeUSD(leg)
			inv.qty += leg.Qty
			inv.cost += leg.Qty * leg.PxUSD
			lot.buyQty += leg.Qty
			lot.buyCost += leg.Qty * leg.PxUSD
			lot.buyFee += fee
			lot.orders = append(lot.orders, spotOrderFromLeg(leg, 0))
			continue
		}
		openLot(at)
		if shortfall := leg.Qty - inv.qty; shortfall > eps {
			addSynthetic(shortfall, at)
		}
		if leg.Quote != "" {
			lot.quote, lot.instId = leg.Quote, leg.InstId
		}
		avg := inv.avg()
		fee := helpers.SpotFeeUSD(leg)
		realized := (leg.PxUSD - avg) * leg.Qty
		inv.qty -= leg.Qty
		inv.cost -= avg * leg.Qty
		lot.sellQty += leg.Qty
		lot.proceeds += leg.Qty * leg.PxUSD
		lot.sellFee += fee
		lot.realized += realized
		lot.lastSell = at
		lot.orders = append(lot.orders, spotOrderFromLeg(leg, realized-fee))
		if dust(inv.qty) {
			closeLot()
		}
	}

	if inv.qty-balance > eps {
		avg := inv.avg()
		inv.qty = balance
		inv.cost = avg * balance
	}
	if dust(balance) {
		if inv.qty > eps {
			closeLot()
		}
		return closed, nil
	}
	if inv.qty <= eps {
		lot = spotLot{}
		addSynthetic(balance, windowStart)
	}
	openLot(windowStart)

	entry := inv.avg()
	if entry <= 0 {
		entry = holding.AvgPx
	}
	id, _ := uuid.NewV7()
	for i := range lot.orders {
		lot.orders[i].PositionID = id
	}
	quote := lot.quote
	if quote == "" {
		quote = "USDT"
	}
	return closed, &domain.OpenPosition{
		ID:           id,
		Pair:         asset + quote,
		Symbol:       asset + "-" + quote,
		Amount:       helpers.Round8(balance),
		Multiplier:   1,
		Side:         "LONG",
		EntryPrice:   helpers.Round8(entry),
		CurrentPrice: helpers.Round8(holding.PriceUSD),
		Commission:   helpers.Round8(lot.buyFee + lot.sellFee),
		RealizedPnl:  helpers.Round8(lot.realized),
		OpenTime:     lot.start,
		Orders:       lot.orders,
		IsSpot:       true,
	}
}

func spotClosedPosition(asset string, lot spotLot) domain.Position {
	id, _ := uuid.NewV7()
	qty := lot.sellQty
	pnl := lot.realized
	entry := (lot.proceeds - pnl) / qty
	exit := lot.proceeds / qty
	fee := lot.buyFee + lot.sellFee
	net := pnl - fee
	closedAt := lot.lastSell

	status := "lose"
	if net > 0 {
		status = "win"
	}

	orders := lot.orders
	for i := range orders {
		orders[i].PositionID = id
	}
	quote := lot.quote
	if quote == "" {
		quote = "USDT"
	}
	instId := lot.instId
	if instId == "" {
		instId = asset + "-" + quote
	}

	return domain.Position{
		ID:         id,
		Side:       "LONG",
		Pair:       asset + quote,
		Symbol:     instId,
		Amount:     helpers.Round8(qty),
		EntryPrice: helpers.Round8(entry),
		ExitPrice:  helpers.Round8(exit),
		Pnl:        helpers.Round8(pnl),
		NetPnl:     helpers.Round8(net),
		Commission: helpers.Round8(fee),
		Multiplier: 1,
		Closed:     true,
		Status:     &status,
		CreatedAt:  lot.start,
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
