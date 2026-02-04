package builders

import (
	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/service/reconstructor/helpers"
	"hyperliquid-trade-reconstructor/internal/service/reconstructor/models"
	"strconv"
	"time"

	"github.com/google/uuid"
)

func BuildPositionFromEnvelope(env models.TradeEnvelope) domain.Position {
	fills := env.Fills

	first := fills[0]
	last := fills[len(fills)-1]

	var amount, fee, pnl float64

	var orders []domain.Order

	positionId := uuid.New()

	for _, f := range fills {
		if helpers.IsOpen(f.Dir) {
			amount += helpers.MustFloat(f.Sz)
		}
		fee += helpers.MustFloat(f.Fee)
		pnl += helpers.MustFloat(f.ClosedPnl)

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
			Amount:            helpers.MustFloat(f.Sz),
			AmountFilled:      helpers.MustFloat(f.Sz),
			AveragePrice:      helpers.MustFloat(f.Px),
			StopPrice:         helpers.MustFloat(f.Px),
			OriginalPrice:     helpers.MustFloat(f.Px),
			UpdatedAt:         time.UnixMilli(f.Time),
		})
	}

	entry := helpers.MustFloat(first.Px)
	exit := helpers.MustFloat(last.Px)

	start := time.UnixMilli(first.Time)
	end := time.UnixMilli(last.Time)

	net := pnl - fee + env.Funding
	status := "Loss"
	if net > 0 {
		status = "Win"
	}

	side := helpers.SideFromDir(first.Dir)

	var mae, mfe *float64
	if env.High != nil && env.Low != nil {
		if side == "Long" {
			maeVal := (*env.Low - entry) * amount
			mfeVal := (*env.High - entry) * amount
			mae = &maeVal
			mfe = &mfeVal
		}
		if side == "Short" {
			maeVal := (entry - *env.High) * amount
			mfeVal := (entry - *env.Low) * amount
			mae = &maeVal
			mfe = &mfeVal
		}

	}

	return domain.Position{
		ID:         positionId,
		UserID:     uint64(uuid.New().ID()),
		KeyID:      uint64(uuid.New().ID()),
		Side:       side,
		Pair:       first.Coin + "/" + first.FeeToken,
		Amount:     amount,
		EntryPrice: entry,
		ExitPrice:  exit,
		Pnl:        pnl,
		NetPnl:     net,
		Commission: fee,
		MAE:        mae,
		MFE:        mfe,
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
