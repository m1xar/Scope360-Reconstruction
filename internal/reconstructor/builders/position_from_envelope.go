package builders

import (
	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/reconstructor"
	"strconv"
	"time"

	"github.com/google/uuid"
)

func BuildPositionFromEnvelope(env domain.TradeEnvelope) domain.Position {
	fills := env.Fills

	first := fills[0]
	last := fills[len(fills)-1]

	var amount, fee, pnl float64

	var orders []domain.Order

	positionId := uuid.New()

	for _, f := range fills {
		if reconstructor.IsOpen(f.Dir) {
			amount += reconstructor.MustFloat(f.Sz)
		}
		fee += reconstructor.MustFloat(f.Fee)
		pnl += reconstructor.MustFloat(f.ClosedPnl)

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
			Amount:            reconstructor.MustFloat(f.Sz),
			AmountFilled:      reconstructor.MustFloat(f.Sz),
			AveragePrice:      reconstructor.MustFloat(f.Px),
			StopPrice:         reconstructor.MustFloat(f.Px),
			OriginalPrice:     reconstructor.MustFloat(f.Px),
			UpdatedAt:         time.Unix(f.Time, 0),
		})
	}

	entry := reconstructor.MustFloat(first.Px)
	exit := reconstructor.MustFloat(last.Px)

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
		Side:       reconstructor.SideFromDir(first.Dir),
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
		Funding:    env.Funding,
	}
}
