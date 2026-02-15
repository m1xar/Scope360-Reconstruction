package builders

import (
	"hyperliquid-trade-reconstructor/internal/domain"
	"hyperliquid-trade-reconstructor/internal/service/reconstructor/helpers"
	"hyperliquid-trade-reconstructor/internal/service/reconstructor/models"
	"strconv"
	"time"

	"github.com/google/uuid"
)

func BuildPositionFromEnvelope(env models.TradeEnvelope) (domain.Position, error) {
	fills := env.Fills

	first := fills[0]
	last := fills[len(fills)-1]

	var amount, fee, pnl float64

	var orders []domain.Order

	newID, err := uuid.NewV7()
	if err != nil {
		return domain.Position{}, err
	}

	newPositionID, err := uuid.NewV7()
	if err != nil {
		return domain.Position{}, err
	}

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
			ID:                newID,
			PositionID:        newPositionID,
			ExchangeOrderID:   strconv.FormatInt(f.Tid, 10),
			Type:              orderType,
			OriginalOrderType: orderType,
			Status:            "Filled",
			Side:              orderType,
			Reduce:            true,
			Amount:            helpers.Round8(helpers.MustFloat(f.Sz)),
			AmountFilled:      helpers.Round8(helpers.MustFloat(f.Sz)),
			AveragePrice:      helpers.Round8(helpers.MustFloat(f.Px)),
			StopPrice:         helpers.Round8(helpers.MustFloat(f.Px)),
			OriginalPrice:     helpers.Round8(helpers.MustFloat(f.Px)),
			UpdatedAt:         time.UnixMilli(f.Time).UTC(),
		})
	}

	entry := helpers.MustFloat(first.Px)
	exit := helpers.MustFloat(last.Px)

	start := time.UnixMilli(first.Time).UTC()
	end := time.UnixMilli(last.Time).UTC()

	net := pnl - fee + env.Funding
	status := "Loss"
	if net > 0 {
		status = "Win"
	}

	side := helpers.SideFromDir(first.Dir)

	var mae, mfe *float64
	if env.High != nil && env.Low != nil {
		if side == "Long" {
			maeVal := helpers.Round8((*env.Low - entry) * amount)
			mfeVal := helpers.Round8((*env.High - entry) * amount)
			mae = &maeVal
			mfe = &mfeVal
		}
		if side == "Short" {
			maeVal := helpers.Round8((entry - *env.High) * amount)
			mfeVal := helpers.Round8((entry - *env.Low) * amount)
			mae = &maeVal
			mfe = &mfeVal
		}

	}

	return domain.Position{
		ID:         newPositionID,
		Side:       side,
		Pair:       first.Coin + first.FeeToken,
		Amount:     helpers.Round8(amount),
		EntryPrice: helpers.Round8(entry),
		ExitPrice:  helpers.Round8(exit),
		Pnl:        helpers.Round8(pnl),
		NetPnl:     helpers.Round8(net),
		Commission: helpers.Round8(fee),
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
		Funding:    helpers.Round8(env.Funding),
	}, nil
}
