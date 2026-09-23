package builders

import (
	"math"

	"github.com/google/uuid"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor/envelope"
	"github.com/m1xar/scope360-reconstruction/pkg/bybit/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/reconstruction/excursion"
)

func BuildPositionFromEnvelope(env envelope.PositionEnvelope) (domain.Position, error) {
	posID, err := uuid.NewV7()
	if err != nil {
		return domain.Position{}, err
	}

	orders := buildOrders(env.Parts, env.Orders, posID)

	var openSize, openNotional, closeSize, closeNotional float64
	var pnl, fee float64
	for i, part := range env.Parts {
		if part.Sign == env.OpenSign {
			openSize += part.Size
			openNotional += part.Size * part.Price
		} else {
			closeSize += part.Size
			closeNotional += part.Size * part.Price
		}
		if i < len(orders) {
			fee += orders[i].Trade.Commission
			pnl += orders[i].Trade.Profit
		}
	}

	if env.FeeRefund != 0 && len(orders) > 0 {
		last := &orders[len(orders)-1]
		last.Trade.Commission = helpers.Round8(last.Trade.Commission - env.FeeRefund)
		fee -= env.FeeRefund
	}

	entry := 0.0
	if openSize > 0 {
		entry = openNotional / openSize
	}
	exit := 0.0
	if closeSize > 0 {
		exit = closeNotional / closeSize
	}

	net := pnl - fee + env.Funding
	status := "lose"
	if net > 0 {
		status = "win"
	}

	closedAt := env.CloseAt
	position := domain.Position{
		ID:               posID,
		Side:             env.Side,
		Pair:             helpers.NormalizePair(env.Symbol),
		Symbol:           env.Symbol,
		Amount:           helpers.Round8(openSize),
		EntryPrice:       helpers.Round8(entry),
		ExitPrice:        helpers.Round8(exit),
		Pnl:              helpers.Round8(pnl),
		NetPnl:           helpers.Round8(net),
		Commission:       helpers.Round8(fee),
		Funding:          helpers.Round8(env.Funding),
		TP:               env.TakeProfit,
		SL:               env.StopLoss,
		LiquidationPrice: liquidationPrice(env, entry),
		Multiplier:       uint32(env.Leverage),
		Isolated:         env.Isolated,
		Closed:           env.Closed,
		Status:           &status,
		CreatedAt:        env.OpenAt,
		ClosedAt:         &closedAt,
		Orders:           orders,
	}

	position.RR, position.RRPlanned = riskReward(env, entry, openSize, net)
	if env.CandlesLoaded {
		position.MAE, position.MFE = excursion.Compute(env.Side, entry, openSize, env.High, env.Low, pnl, net)
	}

	return position, nil
}

func liquidationPrice(env envelope.PositionEnvelope, entry float64) float64 {
	if env.Leverage <= 0 || !env.Isolated {
		return 0
	}
	if env.Side == "LONG" {
		return helpers.Round8(entry * (1 - 1/env.Leverage))
	}
	return helpers.Round8(entry * (1 + 1/env.Leverage))
}

func riskReward(env envelope.PositionEnvelope, entry, size, net float64) (rr, rrPlanned *float64) {
	if env.StopLoss == nil {
		return nil, nil
	}

	slDist := math.Abs(*env.StopLoss-entry) * size
	if slDist == 0 {
		return nil, nil
	}

	rrVal := net / slDist
	rr = &rrVal

	if env.TakeProfit != nil {
		rrpVal := math.Abs(*env.TakeProfit-entry) / math.Abs(*env.StopLoss-entry)
		rrPlanned = &rrpVal
	}
	return rr, rrPlanned
}
