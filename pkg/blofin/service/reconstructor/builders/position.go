package builders

import (
	"math"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/connector/blofin/models"
	"github.com/m1xar/scope360-reconstruction/pkg/blofin/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func BuildPosition(
	hp models.PositionHistory,
	orders []models.Order,
	tpslOrders []models.Order,
	algoOrders []models.Order,
	contractValue float64,
) (domain.Position, error) {
	posID, err := uuid.NewV7()
	if err != nil {
		return domain.Position{}, err
	}
	if contractValue <= 0 {
		contractValue = 1
	}

	entry := helpers.MustFloat(hp.OpenAveragePrice)
	exit := helpers.MustFloat(hp.CloseAveragePrice)
	amount := helpers.MustFloat(hp.MaxPositions) * contractValue
	pnl := helpers.MustFloat(hp.RealizedPnl)
	commission := math.Abs(helpers.MustFloat(hp.Fee))
	net := pnl - commission
	side := helpers.SideFromRaw(hp.Side, hp.PositionSide)
	start := helpers.TimeFromMs(hp.CreateTime)
	end := helpers.TimeFromMs(hp.UpdateTime)
	lever := uint32(helpers.MustFloat(hp.Leverage))

	status := "lose"
	if net > 0 {
		status = "win"
	}

	tp, sl := FindTPSL(orders, tpslOrders, algoOrders)
	rr, rrPlanned := calcRR(net, entry, amount, tp, sl)

	domainOrders := BuildOrders(orders, posID, contractValue)
	if len(domainOrders) == 0 {
		domainOrders = buildSyntheticOrders(hp, posID, contractValue)
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
		Pair:             helpers.NormalizePair(hp.InstID),
		Amount:           helpers.Round8(amount),
		EntryPrice:       helpers.Round8(entry),
		ExitPrice:        helpers.Round8(exit),
		Pnl:              helpers.Round8(pnl),
		NetPnl:           helpers.Round8(net),
		Commission:       helpers.Round8(commission),
		Funding:          0,
		TP:               tp,
		SL:               sl,
		RR:               rr,
		RRPlanned:        rrPlanned,
		LiquidationPrice: helpers.Round8(helpers.MustFloat(hp.LiquidationPrice)),
		Multiplier:       lever,
		Isolated:         hp.MarginMode == "isolated",
		Closed:           true,
		Status:           &status,
		CreatedAt:        start,
		ClosedAt:         &end,
		Orders:           domainOrders,
	}, nil
}

func FindTPSL(orderSets ...[]models.Order) (tp, sl *float64) {
	for _, orders := range orderSets {
		for _, ord := range orders {
			if v := helpers.MustFloat(ord.TpTriggerPrice); v > 0 && tp == nil {
				rounded := helpers.Round8(v)
				tp = &rounded
			}
			if v := helpers.MustFloat(ord.SlTriggerPrice); v > 0 && sl == nil {
				rounded := helpers.Round8(v)
				sl = &rounded
			}
			if tp != nil && sl != nil {
				return tp, sl
			}
		}
	}
	return tp, sl
}

func calcRR(net, entry, amount float64, tp, sl *float64) (rr, rrPlanned *float64) {
	if sl == nil || amount <= 0 {
		return nil, nil
	}
	slDist := math.Abs(*sl-entry) * amount
	if slDist <= 0 {
		return nil, nil
	}
	rrVal := helpers.Round8(net / slDist)
	rr = &rrVal
	if tp != nil {
		rrpVal := helpers.Round8(math.Abs(*tp-entry) / math.Abs(*sl-entry))
		rrPlanned = &rrpVal
	}
	return rr, rrPlanned
}

func buildSyntheticOrders(hp models.PositionHistory, posID uuid.UUID, contractValue float64) []domain.Order {
	if contractValue <= 0 {
		contractValue = 1
	}
	entry := helpers.MustFloat(hp.OpenAveragePrice)
	exit := helpers.MustFloat(hp.CloseAveragePrice)
	amount := helpers.MustFloat(hp.MaxPositions) * contractValue
	fee := math.Abs(helpers.MustFloat(hp.Fee))
	pnl := helpers.MustFloat(hp.RealizedPnl)
	openTime := helpers.TimeFromMs(hp.CreateTime)
	closeTime := helpers.TimeFromMs(hp.UpdateTime)
	side := helpers.SideFromRaw(hp.Side, hp.PositionSide)

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
	amount := pos.Amount
	if pos.Side == "LONG" {
		maeVal := helpers.Round8((*low - entry) * amount)
		mfeVal := helpers.Round8((*high - entry) * amount)
		pos.MAE = &maeVal
		pos.MFE = &mfeVal
		return
	}
	maeVal := helpers.Round8((entry - *high) * amount)
	mfeVal := helpers.Round8((entry - *low) * amount)
	pos.MAE = &maeVal
	pos.MFE = &mfeVal
}
