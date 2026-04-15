package builders

import (
	"math"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
)

func BuildPosition(
	hp models.HistoryPosition,
	orders []models.Order,
	funding float64,
) (domain.Position, error) {
	posID, err := uuid.NewV7()
	if err != nil {
		return domain.Position{}, err
	}

	entry := hp.OpenAvgPrice
	exit := hp.CloseAvgPrice
	amount := hp.CloseVol
	pnl := hp.Realised
	side := helpers.SideFromPositionType(hp.PositionType)
	start := helpers.TimeFromMs(hp.CreateTime)
	end := helpers.TimeFromMs(hp.UpdateTime)

	var commission float64
	if len(orders) > 0 {
		for _, ord := range orders {
			commission += math.Abs(ord.TakerFee) + math.Abs(ord.MakerFee)
		}
	} else {
		commission = math.Abs(hp.HoldFee)
	}

	net := helpers.Round8(pnl - commission + funding)

	status := "lose"
	if net > 0 {
		status = "win"
	}

	lever := uint32(hp.Leverage)

	var sl, tp *float64
	for _, ord := range orders {
		if ord.DealVol <= 0 {
			continue
		}
		if ord.StopLossPrice > 0 && sl == nil {
			v := helpers.Round8(ord.StopLossPrice)
			sl = &v
		}
		if ord.TakeProfitPrice > 0 && tp == nil {
			v := helpers.Round8(ord.TakeProfitPrice)
			tp = &v
		}
	}

	var rr, rrPlanned *float64
	if sl != nil && amount > 0 {
		slDist := math.Abs(*sl-entry) * amount
		if slDist > 0 {
			rrVal := net / slDist
			rr = &rrVal
			if tp != nil {
				rrpVal := math.Abs(*tp-entry) / math.Abs(*sl-entry)
				rrPlanned = &rrpVal
			}
		}
	}

	var liqPrice float64
	if lever > 0 && hp.OpenType == 1 {
		if hp.LiquidatePrice > 0 {
			liqPrice = helpers.Round8(hp.LiquidatePrice)
		} else {
			if side == "LONG" {
				liqPrice = helpers.Round8(entry * (1 - 1/float64(lever)))
			} else {
				liqPrice = helpers.Round8(entry * (1 + 1/float64(lever)))
			}
		}
	}

	var domainOrders []domain.Order
	if len(orders) > 0 {
		domainOrders = BuildOrders(orders, posID)
	}
	if len(domainOrders) == 0 {
		domainOrders = buildSyntheticOrders(hp, posID)
	}

	if len(domainOrders) > 0 {
		var orderFeeSum float64
		for _, o := range domainOrders {
			orderFeeSum += o.Trade.Commission
		}
		if gap := helpers.Round8(commission - orderFeeSum); gap != 0 {
			last := &domainOrders[len(domainOrders)-1]
			last.Trade.Commission = helpers.Round8(last.Trade.Commission + gap)
		}
	}

	return domain.Position{
		ID:               posID,
		Side:             side,
		Pair:             helpers.NormalizePair(hp.Symbol),
		Amount:           helpers.Round8(amount),
		EntryPrice:       helpers.Round8(entry),
		ExitPrice:        helpers.Round8(exit),
		Pnl:              helpers.Round8(pnl),
		NetPnl:           helpers.Round8(net),
		Commission:       helpers.Round8(commission),
		Funding:          helpers.Round8(funding),
		MAE:              nil,
		MFE:              nil,
		TP:               tp,
		SL:               sl,
		RR:               rr,
		RRPlanned:        rrPlanned,
		LiquidationPrice: liqPrice,
		Isolated:         hp.OpenType == 1,
		Closed:           true,
		Status:           &status,
		Multiplier:       lever,
		CreatedAt:        start,
		ClosedAt:         &end,
		Orders:           domainOrders,
	}, nil
}

func BuildOrders(orders []models.Order, posID uuid.UUID) []domain.Order {
	result := make([]domain.Order, 0, len(orders))

	for _, ord := range orders {
		if ord.DealVol <= 0 {
			continue
		}

		orderID, err := uuid.NewV7()
		if err != nil {
			continue
		}

		side := helpers.OrderSideFromMEXC(ord.Side)
		avgPx := helpers.Round8(ord.DealAvgPrice)
		fillSz := helpers.Round8(ord.DealVol)
		fee := helpers.Round8(math.Abs(ord.TakerFee) + math.Abs(ord.MakerFee))
		profit := helpers.Round8(ord.Profit)
		doneAt := helpers.TimeFromMs(ord.UpdateTime)

		result = append(result, domain.Order{
			ID:              orderID,
			PositionID:      posID,
			ExchangeOrderID: ord.OrderId,
			Type:            helpers.OrderTypeFromMEXC(ord.OrderType),
			Status:          "FILLED",
			Side:            side,
			Amount:          helpers.Round8(ord.DealVol),
			AmountFilled:    fillSz,
			AveragePrice:    avgPx,
			StopPrice:       helpers.Round8(ord.StopLossPrice),
			OriginalPrice:   helpers.Round8(ord.Price),
			UpdatedAt:       doneAt,
			Trade: domain.Trade{
				OrderID:    orderID,
				Side:       side,
				Price:      avgPx,
				Amount:     fillSz,
				Commission: fee,
				Profit:     profit,
				DoneAt:     doneAt,
			},
		})
	}

	return result
}

func buildSyntheticOrders(hp models.HistoryPosition, posID uuid.UUID) []domain.Order {
	entry := hp.OpenAvgPrice
	exit := hp.CloseAvgPrice
	amount := hp.CloseVol
	fee := math.Abs(hp.HoldFee)
	pnl := hp.Realised
	openTime := helpers.TimeFromMs(hp.CreateTime)
	closeTime := helpers.TimeFromMs(hp.UpdateTime)
	side := helpers.SideFromPositionType(hp.PositionType)

	openSide := "BUY"
	closeSide := "SELL"
	if side == "SHORT" {
		openSide = "SELL"
		closeSide = "BUY"
	}

	openID, _ := uuid.NewV7()
	closeID, _ := uuid.NewV7()
	halfFee := helpers.Round8(fee / 2)

	return []domain.Order{
		{
			ID:            openID,
			PositionID:    posID,
			Type:          "MARKET",
			Status:        "FILLED",
			Side:          openSide,
			Amount:        helpers.Round8(amount),
			AmountFilled:  helpers.Round8(amount),
			AveragePrice:  helpers.Round8(entry),
			OriginalPrice: helpers.Round8(entry),
			UpdatedAt:     openTime,
			Trade: domain.Trade{
				OrderID:    openID,
				Side:       openSide,
				Price:      helpers.Round8(entry),
				Amount:     helpers.Round8(amount),
				Commission: halfFee,
				Profit:     0,
				DoneAt:     openTime,
			},
		},
		{
			ID:            closeID,
			PositionID:    posID,
			Type:          "MARKET",
			Status:        "FILLED",
			Side:          closeSide,
			Amount:        helpers.Round8(amount),
			AmountFilled:  helpers.Round8(amount),
			AveragePrice:  helpers.Round8(exit),
			OriginalPrice: helpers.Round8(exit),
			UpdatedAt:     closeTime,
			Trade: domain.Trade{
				OrderID:    closeID,
				Side:       closeSide,
				Price:      helpers.Round8(exit),
				Amount:     helpers.Round8(amount),
				Commission: halfFee,
				Profit:     helpers.Round8(pnl),
				DoneAt:     closeTime,
			},
		},
	}
}

func ApplyMAEMFE(pos *domain.Position, high, low *float64) {
	if high == nil || low == nil {
		return
	}
	entry := pos.EntryPrice
	exit := pos.ExitPrice

	amount := pos.Amount
	priceDelta := exit - entry
	if pos.Side == "SHORT" {
		priceDelta = entry - exit
	}
	if priceDelta != 0 {
		amount = math.Abs(pos.Pnl / priceDelta)
	}

	if pos.Side == "LONG" {
		maeVal := helpers.Round8((*low - entry) * amount)
		mfeVal := helpers.Round8((*high - entry) * amount)
		pos.MAE = &maeVal
		pos.MFE = &mfeVal
	} else {
		maeVal := helpers.Round8((entry - *high) * amount)
		mfeVal := helpers.Round8((entry - *low) * amount)
		pos.MAE = &maeVal
		pos.MFE = &mfeVal
	}
}
