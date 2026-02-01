package reconstructor

import (
	"hyperliquid-trade-reconstructor/internal/domain"
	"strconv"
	"time"

	"github.com/google/uuid"
)

func BuildPositionFromEnvelope(env TradeEnvelope) domain.Position {
	fills := env.Fills

	first := fills[0]
	last := fills[len(fills)-1]

	var amount, fee, pnl float64

	var orders []domain.Order

	positionId := uuid.New()

	for _, f := range fills {
		if isOpen(f.Dir) {
			amount += mustFloat(f.Sz)
		}
		fee += mustFloat(f.Fee)
		pnl += mustFloat(f.ClosedPnl)

		orderType := "Sell"

		if f.Side == "B" {
			orderType = "Buy"
		}

		orders = append(orders, domain.Order{
			ID:                uuid.New(),
			PositionID:        positionId,
			ExchangeOrderID:   strconv.FormatInt(f.Tid, 10),
			Type:              orderType,
			OriginalOrderType: orderType,
			Status:            "Filled",
			Side:              orderType,
			Reduce:            true,
			Amount:            mustFloat(f.Sz),
			AmountFilled:      mustFloat(f.Sz),
			AveragePrice:      mustFloat(f.Px),
			StopPrice:         mustFloat(f.Px),
			OriginalPrice:     mustFloat(f.Px),
			UpdatedAt:         time.Unix(f.Time, 0),
		})
	}

	entry := mustFloat(first.Px)
	exit := mustFloat(last.Px)

	start := time.UnixMilli(first.Time)
	end := time.UnixMilli(last.Time)

	net := pnl - fee
	status := "Loss"
	if net > 0 {
		status = "Win"
	}

	return domain.Position{
		ID:         positionId,
		UserID:     uint64(uuid.New().ID()),
		KeyID:      uint64(uuid.New().ID()),
		Side:       sideFromDir(first.Dir),
		Pair:       first.Coin + "/" + first.FeeToken,
		Amount:     amount,
		EntryPrice: entry,
		ExitPrice:  exit,
		Pnl:        pnl,
		NetPnl:     net,
		Commission: fee,
		TP:         env.TakeProfit,
		SL:         env.StopLoss,
		Isolated:   true,
		Closed:     true,
		Status:     &status,
		CreatedAt:  start,
		ClosedAt:   &end,
		Orders:     orders,
		Editable:   domain.JSONMap{"editable": true},
	}
}
