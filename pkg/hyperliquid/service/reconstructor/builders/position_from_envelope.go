package builders

import (
	"math"
	"strconv"
	"time"

	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/hyperliquid/service/reconstructor/helpers"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/excursion"
)

func BuildPositionFromEnvelope(env envelope.TradeEnvelope) (domain.Position, error) {
	fills := env.Fills

	first := fills[0]
	last := fills[len(fills)-1]

	var amount, fee, pnl float64

	var orders []domain.Order

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

		trade := domain.Trade{
			OrderID:    newID,
			Side:       side,
			Price:      helpers.Round8(helpers.MustFloat(f.Px)),
			Amount:     helpers.Round8(helpers.MustFloat(f.Sz)),
			Commission: helpers.Round8(helpers.MustFloat(f.Fee)),
			Profit:     helpers.Round8(helpers.MustFloat(f.ClosedPnl)),
			DoneAt:     time.UnixMilli(f.Time).UTC(),
		}

		orders = append(orders, domain.Order{
			ID:              newID,
			PositionID:      newPositionID,
			ExchangeOrderID: strconv.FormatInt(f.Tid, 10),
			Type:            env.FillTypes[f.Tid],
			Status:          "FILLED",
			Side:            side,
			Amount:          helpers.Round8(helpers.MustFloat(f.Sz)),
			AmountFilled:    helpers.Round8(helpers.MustFloat(f.Sz)),
			AveragePrice:    helpers.Round8(helpers.MustFloat(f.Px)),
			StopPrice:       helpers.Round8(helpers.MustFloat(f.Px)),
			OriginalPrice:   helpers.Round8(helpers.MustFloat(f.Px)),
			UpdatedAt:       time.UnixMilli(f.Time).UTC(),
			Trade:           trade,
		})
	}

	entry := helpers.MustFloat(first.Px)
	exit := helpers.MustFloat(last.Px)

	start := time.UnixMilli(first.Time).UTC()
	end := time.UnixMilli(last.Time).UTC()

	net := pnl - fee + env.Funding
	status := "lose"
	if net > 0 {
		status = "win"
	}

	side := helpers.PositionSideFromDir(first.Dir)

	var mae, mfe *float64
	if env.CandlesLoaded {
		mae, mfe = excursion.Compute(side, entry, amount, env.High, env.Low, pnl, net)
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
		Funding:    helpers.Round8(env.Funding),
		RR:         RR,
		RRPlanned:  RRPlanned,
	}, nil
}
