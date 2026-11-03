package builders

import (
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/okx/service/reconstructor/helpers"
)

const spotDustUSD = 0.10

// BuildSpotPositions reconstructs the spot history of every asset as lots and
// groups all lots of an asset into a single position. A lot opens with the
// first buy (or a synthetic basis when history starts mid-lot), absorbs every
// buy and sell as an execution and closes when the remainder is dust: below
// the instrument's minimum order size, under spotDustUSD or effectively zero.
//
// Open or closed is decided by the live balance alone. An asset that is still
// held becomes one open position carrying every execution of all its lots, the
// realized pnl of all sells so far and the entry price of the live lot. An
// asset that is no longer held becomes one closed position over all its lots:
// volume-weighted entry/exit, summed pnl and fees, opened at the first lot's
// start and closed at the last sell; buys whose units left without a sell stay
// among its executions. Each sell keeps its own realized pnl in Trade.Profit.
//
// Every balance adjustment is replayed as an execution on its day (see
// spotAdjustmentLeg) with status helpers.OrderStatusAdjustment and the
// adjustment Key as exchange order id. Only what those executions still leave
// unexplained becomes a helpers.OrderStatusSynthetic buy: at the window start
// for the opening inventory, or at a sell that would otherwise run short. Such
// synthetics carry helpers.SpotSyntheticKey as exchange order id. The open
// quantity never exceeds the live balance. now caps adjustment sells of the
// current day so no position closes in the future.
func BuildSpotPositions(legs []helpers.SpotLeg, holdings []helpers.SpotHolding, windowStart, now time.Time, priceAt helpers.PriceAt, minSizes map[string]float64, adjustments []helpers.SpotAdjustment) ([]domain.Position, []domain.OpenPosition) {
	byAsset := map[string][]helpers.SpotLeg{}
	for _, leg := range legs {
		if helpers.IsStableAsset(leg.Asset) {
			continue
		}
		byAsset[leg.Asset] = append(byAsset[leg.Asset], leg)
	}
	for _, adj := range adjustments {
		if adj.Asset == "" || adj.Qty == 0 || helpers.IsStableAsset(adj.Asset) {
			continue
		}
		byAsset[adj.Asset] = append(byAsset[adj.Asset], spotAdjustmentLeg(adj, priceAt, windowStart, now))
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
		lots := buildSpotLots(asset, byAsset[asset], held[asset], windowStart, priceAt, minSizes[asset])
		c, o := lots.grouped(asset)
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
	start    time.Time
	orders   []domain.Order
	buyFee   float64
	sellQty  float64
	proceeds float64
	sellFee  float64
	realized float64
	lastSell time.Time
	quote    string
	instId   string
}

func (lot spotLot) market(asset string) (pair, symbol string) {
	quote := lot.quote
	if quote == "" {
		quote = "USDT"
	}
	symbol = lot.instId
	if symbol == "" {
		symbol = asset + "-" + quote
	}
	return asset + quote, symbol
}

// spotAssetLots is the lot reconstruction of one asset: every finished lot
// that holds at least one execution, in chronological order, and the live lot
// with its remaining balance, entry price and market price when the asset is
// held.
type spotAssetLots struct {
	lots   []spotLot
	open   *spotLot
	amount float64
	entry  float64
	price  float64
}

func (l spotAssetLots) grouped(asset string) ([]domain.Position, *domain.OpenPosition) {
	lots := append([]spotLot(nil), l.lots...)
	if l.open != nil {
		lots = append(lots, *l.open)
	}
	if len(lots) == 0 {
		return nil, nil
	}
	merged := mergeSpotLots(lots)
	if l.open != nil {
		return nil, l.openPosition(asset, merged)
	}
	if merged.sellQty <= 0 {
		return nil, nil
	}
	return []domain.Position{spotClosedPosition(asset, merged)}, nil
}

func (l spotAssetLots) openPosition(asset string, lot spotLot) *domain.OpenPosition {
	id, _ := uuid.NewV7()
	orders := append([]domain.Order(nil), lot.orders...)
	for i := range orders {
		orders[i].PositionID = id
	}
	pair, symbol := lot.market(asset)
	return &domain.OpenPosition{
		ID:           id,
		Pair:         pair,
		Symbol:       symbol,
		Amount:       helpers.Round8(l.amount),
		Multiplier:   1,
		Side:         "LONG",
		EntryPrice:   helpers.Round8(l.entry),
		CurrentPrice: helpers.Round8(l.price),
		Commission:   helpers.Round8(lot.buyFee + lot.sellFee),
		RealizedPnl:  helpers.Round8(lot.realized),
		OpenTime:     lot.start,
		Orders:       orders,
		IsSpot:       true,
	}
}

// mergeSpotLots folds the lots of one asset into a single lot: executions of
// all lots in time order, summed fees, sold quantity, proceeds and realized
// pnl, the earliest start, the latest sell and the market of the latest lot
// (by start) that traded one.
func mergeSpotLots(lots []spotLot) spotLot {
	lots = append([]spotLot(nil), lots...)
	sort.SliceStable(lots, func(i, j int) bool { return lots[i].start.Before(lots[j].start) })
	var merged spotLot
	for _, lot := range lots {
		if !lot.start.IsZero() && (merged.start.IsZero() || lot.start.Before(merged.start)) {
			merged.start = lot.start
		}
		merged.orders = append(merged.orders, lot.orders...)
		merged.buyFee += lot.buyFee
		merged.sellQty += lot.sellQty
		merged.proceeds += lot.proceeds
		merged.sellFee += lot.sellFee
		merged.realized += lot.realized
		if lot.lastSell.After(merged.lastSell) {
			merged.lastSell = lot.lastSell
		}
		if lot.quote != "" {
			merged.quote, merged.instId = lot.quote, lot.instId
		}
	}
	sort.SliceStable(merged.orders, func(i, j int) bool {
		return merged.orders[i].Trade.DoneAt.Before(merged.orders[j].Trade.DoneAt)
	})
	return merged
}

// spotAdjustmentLeg turns the net balance change of one asset and UTC day into
// an execution at that day's USD price: a gain is a buy at the start of the
// day (00:00:00.000 UTC), a loss a sell at its end (23:59:59.999 UTC), so the
// fills and transfers of that day never run short because of it. A loss on
// the current day is placed at now, so no position closes in the future, and
// no leg is placed before the window start.
func spotAdjustmentLeg(adj helpers.SpotAdjustment, priceAt helpers.PriceAt, windowStart, now time.Time) helpers.SpotLeg {
	day := helpers.SpotDayStart(adj.Ts)
	leg := helpers.SpotLeg{
		Asset:      adj.Asset,
		OrdId:      adj.Key(),
		OrdType:    "market",
		Side:       "buy",
		Qty:        adj.Qty,
		Ts:         day,
		Adjustment: true,
	}
	if adj.Qty < 0 {
		leg.Side, leg.Qty = "sell", -adj.Qty
		leg.Ts = day + helpers.SpotDayMs - 1
		if nowMs := now.UnixMilli(); !now.IsZero() && nowMs > day && nowMs < leg.Ts {
			leg.Ts = nowMs
		}
	}
	if start := windowStart.UnixMilli(); !windowStart.IsZero() && leg.Ts < start {
		leg.Ts = start
	}
	if priceAt != nil {
		if px, ok := priceAt(adj.Asset, leg.Ts); ok {
			leg.PxUSD = px
		}
	}
	return leg
}

func buildSpotLots(asset string, legs []helpers.SpotLeg, holding helpers.SpotHolding, windowStart time.Time, priceAt helpers.PriceAt, minSize float64) spotAssetLots {
	sort.SliceStable(legs, func(i, j int) bool { return legs[i].Ts < legs[j].Ts })

	var buyQty, buyCost, sellQty float64
	for _, leg := range legs {
		if leg.Side == "buy" {
			buyQty += leg.Qty - leg.FeeBase
			buyCost += (leg.Qty - leg.FeeBase) * leg.PxUSD
		} else {
			sellQty += leg.Qty
		}
	}

	balance := holding.Qty
	eps := math.Max(balance, buyQty) * 1e-9

	var inv spotInventory
	var lot spotLot
	var out spotAssetLots
	syntheticKeys := map[string]int{}

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
	addSynthetic := func(qty, px float64, at time.Time) {
		if px <= 0 {
			px = syntheticPrice(qty)
		}
		inv.qty += qty
		inv.cost += qty * px
		if lot.start.IsZero() {
			lot.start = at
		}
		key := helpers.SpotSyntheticKey(asset, at.UnixMilli())
		n := syntheticKeys[key]
		syntheticKeys[key] = n + 1
		if n > 0 {
			key += "#" + strconv.Itoa(n)
		}
		lot.orders = append(lot.orders, syntheticSpotOrder(key, qty, px, at))
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
		if len(lot.orders) > 0 {
			out.lots = append(out.lots, lot)
		}
		if inv.qty <= eps {
			inv = spotInventory{}
		}
		lot = spotLot{}
	}

	if missing := balance - buyQty + sellQty; missing > eps {
		addSynthetic(missing, 0, windowStart)
	}

	for _, leg := range legs {
		at := time.UnixMilli(leg.Ts).UTC()
		if lot.start.IsZero() {
			lot.start = at
		}
		if leg.Quote != "" {
			lot.quote, lot.instId = leg.Quote, leg.InstId
		}
		fee := helpers.SpotFeeUSD(leg)
		if leg.Side == "buy" {
			if leg.PxUSD <= 0 {
				leg.PxUSD = firstPositive(inv.avg(), holding.AvgPx, holding.PriceUSD)
			}
			inv.qty += leg.Qty - leg.FeeBase
			inv.cost += (leg.Qty - leg.FeeBase) * leg.PxUSD
			lot.buyFee += fee
			lot.orders = append(lot.orders, spotOrderFromLeg(leg, 0))
			continue
		}
		if shortfall := leg.Qty - inv.qty; shortfall > eps {
			px := leg.PxUSD
			if px <= 0 && priceAt != nil {
				px, _ = priceAt(asset, leg.Ts)
			}
			addSynthetic(shortfall, px, at)
		}
		avg := inv.avg()
		if leg.PxUSD <= 0 {
			leg.PxUSD = firstPositive(avg, holding.PriceUSD)
		}
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
		closeLot()
		return out
	}

	entry := inv.avg()
	if entry <= 0 {
		entry = holding.AvgPx
	}
	out.open = &lot
	out.amount, out.entry, out.price = balance, entry, holding.PriceUSD
	return out
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

	orders := append([]domain.Order(nil), lot.orders...)
	for i := range orders {
		orders[i].PositionID = id
	}
	pair, symbol := lot.market(asset)

	return domain.Position{
		ID:         id,
		Side:       "LONG",
		Pair:       pair,
		Symbol:     symbol,
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
	switch {
	case leg.Transfer:
		status = helpers.OrderStatusTransfer
	case leg.Adjustment:
		status = helpers.OrderStatusAdjustment
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

func firstPositive(values ...float64) float64 {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func syntheticSpotOrder(key string, qty, px float64, at time.Time) domain.Order {
	id, _ := uuid.NewV7()
	return domain.Order{
		ID:              id,
		ExchangeOrderID: key,
		Type:            "MARKET",
		Status:          helpers.OrderStatusSynthetic,
		Side:            "BUY",
		Amount:          helpers.Round8(qty),
		AmountFilled:    helpers.Round8(qty),
		AveragePrice:    helpers.Round8(px),
		OriginalPrice:   helpers.Round8(px),
		UpdatedAt:       at,
		Trade: domain.Trade{
			OrderID: id,
			Side:    "BUY",
			Price:   helpers.Round8(px),
			Amount:  helpers.Round8(qty),
			DoneAt:  at,
		},
	}
}
