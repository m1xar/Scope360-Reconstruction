package builders

import (
	"math"
	"strconv"
	"time"

	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/helpers"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/hyperliquid/service/reconstructor/models"

	"github.com/google/uuid"
)

func BuildPositionFromEnvelope(env models.TradeEnvelope) (domain.Position, error) {
	fills := env.Fills

	first := fills[0]
	last := fills[len(fills)-1]

	var amount, fee, pnl float64

	var orders []domain.Order
	var trades []domain.Trade

	newPositionID, err := uuid.NewV7()
	if err != nil {
		return domain.Position{}, err
	}

	for _, f := range fills {

		newID, err := uuid.NewV7()
		if err != nil {
			return domain.Position{}, err
		}

		if helpers.IsOpen(f.Dir) {
			amount += helpers.MustFloat(f.Sz)
		}
		fee += helpers.MustFloat(f.Fee)
		pnl += helpers.MustFloat(f.ClosedPnl)

		side := "SELL"

		if f.Side == "B" {
			side = "BUY"
		}

		orders = append(orders, domain.Order{
			ID:                newID,
			PositionID:        newPositionID,
			ExchangeOrderID:   strconv.FormatInt(f.Tid, 10),
			Type:              env.FillTypes[f.Tid],
			OriginalOrderType: env.FillTypes[f.Tid],
			Status:            "Filled",
			Side:              side,
			Reduce:            true,
			Amount:            helpers.Round8(helpers.MustFloat(f.Sz)),
			AmountFilled:      helpers.Round8(helpers.MustFloat(f.Sz)),
			AveragePrice:      helpers.Round8(helpers.MustFloat(f.Px)),
			StopPrice:         helpers.Round8(helpers.MustFloat(f.Px)),
			OriginalPrice:     helpers.Round8(helpers.MustFloat(f.Px)),
			UpdatedAt:         time.UnixMilli(f.Time).UTC(),
		})

		trades = append(trades, domain.Trade{
			OrderID:    newID,
			Side:       side,
			Price:      helpers.Round8(helpers.MustFloat(f.Px)),
			Amount:     helpers.Round8(helpers.MustFloat(f.Sz)),
			Commission: helpers.Round8(helpers.MustFloat(f.Fee)),
			Profit:     helpers.Round8(helpers.MustFloat(f.ClosedPnl)),
			DoneAt:     time.UnixMilli(f.Time).UTC(),
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
		if side == "BUY" {
			maeVal := helpers.Round8((*env.Low - entry) * amount)
			mfeVal := helpers.Round8((*env.High - entry) * amount)
			mae = &maeVal
			mfe = &mfeVal
		}
		if side == "SELL" {
			maeVal := helpers.Round8((entry - *env.High) * amount)
			mfeVal := helpers.Round8((entry - *env.Low) * amount)
			mae = &maeVal
			mfe = &mfeVal
		}

	}
	var RR, RRPlanned *float64
	if env.StopLoss != nil {
		rr := (net / (math.Abs((*env.StopLoss - entry)) * amount))
		RR = &rr
		if env.TakeProfit != nil {
			rrplanned := math.Abs(*env.TakeProfit-entry) / math.Abs(*env.StopLoss-entry)
			RRPlanned = &rrplanned
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
		Trades:     trades,
		Funding:    helpers.Round8(env.Funding),
		RR:         RR,
		RRPlanned:  RRPlanned,
	}, nil
}
