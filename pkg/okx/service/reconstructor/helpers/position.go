package helpers

import (
	"math"
	"strconv"

	"github.com/google/uuid"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/okx/connector/okx/models"
)

func BuildPosition(
	cp models.ClosedPosition,
	orders []models.Order,
) (domain.Position, error) {
	posID, err := uuid.NewV7()
	if err != nil {
		return domain.Position{}, err
	}

	entry := MustFloat(cp.OpenAvgPx)
	exit := MustFloat(cp.CloseAvgPx)
	amount := MustFloat(cp.OpenMaxPos)
	pnl := MustFloat(cp.Pnl)
	fee := math.Abs(MustFloat(cp.Fee))
	funding := MustFloat(cp.FundingFee)
	net := MustFloat(cp.RealizedPnl)

	side := SideFromDirection(cp.Direction)
	start := TimeFromMs(cp.CTime)
	end := TimeFromMs(cp.UTime)

	status := "lose"
	if net > 0 {
		status = "win"
	}

	lever, _ := strconv.ParseUint(cp.Lever, 10, 32)

	var sl, tp *float64
	for _, ord := range orders {
		if !IsFilled(ord) {
			continue
		}
		if v := MustFloat(ord.SlTriggerPx); v > 0 && sl == nil {
			rounded := Round8(v)
			sl = &rounded
		}
		if v := MustFloat(ord.TpTriggerPx); v > 0 && tp == nil {
			rounded := Round8(v)
			tp = &rounded
		}
	}

	var rr, rrPlanned *float64
	if sl != nil {
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

	domainOrders := BuildOrders(orders, posID)

	return domain.Position{
		ID:         posID,
		Side:       side,
		Pair:       NormalizePair(cp.InstId),
		Amount:     Round8(amount),
		EntryPrice: Round8(entry),
		ExitPrice:  Round8(exit),
		Pnl:        Round8(pnl),
		NetPnl:     Round8(net),
		Commission: Round8(fee),
		Funding:    Round8(funding),
		MAE:        nil,
		MFE:        nil,
		TP:         tp,
		SL:         sl,
		RR:         rr,
		RRPlanned:  rrPlanned,
		Isolated:   cp.MgnMode == "isolated",
		Closed:     true,
		Status:     &status,
		Multiplier: uint32(lever),
		CreatedAt:  start,
		ClosedAt:   &end,
		Orders:     domainOrders,
	}, nil
}

func BuildOrders(orders []models.Order, posID uuid.UUID) []domain.Order {
	result := make([]domain.Order, 0, len(orders))

	for _, ord := range orders {
		if !IsFilled(ord) {
			continue
		}

		orderID, err := uuid.NewV7()
		if err != nil {
			continue
		}

		side := OrderSideFromOKX(ord.Side)
		avgPx := Round8(MustFloat(ord.AvgPx))
		fillSz := Round8(MustFloat(ord.AccFillSz))
		fee := Round8(math.Abs(MustFloat(ord.Fee)))
		pnl := Round8(MustFloat(ord.Pnl))
		doneAt := TimeFromMs(ord.UTime)

		result = append(result, domain.Order{
			ID:              orderID,
			PositionID:      posID,
			ExchangeOrderID: ord.OrdId,
			Type:            OrderTypeFromOKX(ord.OrdType),
			Status:          "FILLED",
			Side:            side,
			Amount:          Round8(MustFloat(ord.Sz)),
			AmountFilled:    fillSz,
			AveragePrice:    avgPx,
			StopPrice:       Round8(MustFloat(ord.SlTriggerPx)),
			OriginalPrice:   Round8(MustFloat(ord.Px)),
			UpdatedAt:       doneAt,
			Trade: domain.Trade{
				OrderID:    orderID,
				Side:       side,
				Price:      avgPx,
				Amount:     fillSz,
				Commission: fee,
				Profit:     pnl,
				DoneAt:     doneAt,
			},
		})
	}

	return result
}

func ApplyMAEMFE(pos *domain.Position, high, low *float64) {
	if high == nil || low == nil {
		return
	}
	entry := pos.EntryPrice
	amount := pos.Amount
	if pos.Side == "LONG" {
		maeVal := Round8((*low - entry) * amount)
		mfeVal := Round8((*high - entry) * amount)
		pos.MAE = &maeVal
		pos.MFE = &mfeVal
	} else {
		maeVal := Round8((entry - *high) * amount)
		mfeVal := Round8((entry - *low) * amount)
		pos.MAE = &maeVal
		pos.MFE = &mfeVal
	}
}
