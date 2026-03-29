package builders

import (
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/domain"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/service/reconstructor/envelope"
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/service/reconstructor/helpers"
)

func BuildPositionFromEnvelope(env envelope.TradeEnvelope) (domain.Position, error) {
	fills := env.Fills
	if len(fills) == 0 {
		return domain.Position{}, nil
	}

	first := fills[0]
	last := fills[len(fills)-1]

	positionID, err := uuid.NewV7()
	if err != nil {
		return domain.Position{}, err
	}

	var amount, fee, pnl float64
	var openNotional float64
	var orders []domain.Order

	for _, f := range fills {
		orderID, err := uuid.NewV7()
		if err != nil {
			return domain.Position{}, err
		}

		isClose := f.RealizedPnl != 0

		if !isClose {
			amount += f.ExecutedQuantity
			openNotional += f.ExecutedQuantity * f.ExecutedPrice
		}

		fee += math.Abs(f.Fee)
		pnl += f.RealizedPnl

		fillType := "MARKET"
		if ft, ok := env.FillTypes[f.ID]; ok {
			fillType = ft
		}

		trade := domain.Trade{
			OrderID:    orderID,
			Side:       f.Side,
			Price:      helpers.Round8(f.ExecutedPrice),
			Amount:     helpers.Round8(f.ExecutedQuantity),
			Commission: helpers.Round8(math.Abs(f.Fee)),
			Profit:     helpers.Round8(f.RealizedPnl),
			DoneAt:     time.UnixMilli(f.ExecutedTimestamp).UTC(),
		}

		orders = append(orders, domain.Order{
			ID:              orderID,
			PositionID:      positionID,
			ExchangeOrderID: strconv.Itoa(f.ID),
			Type:            fillType,
			Status:          "FILLED",
			Side:            f.Side,
			Amount:          helpers.Round8(f.ExecutedQuantity),
			AmountFilled:    helpers.Round8(f.ExecutedQuantity),
			AveragePrice:    helpers.Round8(f.ExecutedPrice),
			StopPrice:       helpers.Round8(f.ExecutedPrice),
			OriginalPrice:   helpers.Round8(f.ExecutedPrice),
			UpdatedAt:       time.UnixMilli(f.ExecutedTimestamp).UTC(),
			Trade:           trade,
		})
	}

	entry := 0.0
	if amount > 0 {
		entry = openNotional / amount
	}
	exit := last.ExecutedPrice

	start := time.UnixMilli(first.ExecutedTimestamp).UTC()
	end := time.UnixMilli(last.ExecutedTimestamp).UTC()

	net := pnl - fee + env.Funding
	status := "lose"
	if net > 0 {
		status = "win"
	}

	side := env.Side

	var mae, mfe *float64
	if env.High != nil && env.Low != nil {
		if side == "LONG" {
			maeVal := helpers.Round8((*env.Low - entry) * amount)
			mfeVal := helpers.Round8((*env.High - entry) * amount)
			mae = &maeVal
			mfe = &mfeVal
		}
		if side == "SHORT" {
			maeVal := helpers.Round8((entry - *env.High) * amount)
			mfeVal := helpers.Round8((entry - *env.Low) * amount)
			mae = &maeVal
			mfe = &mfeVal
		}
	}

	var rr, rrPlanned *float64
	if env.StopLoss != nil {
		slDist := math.Abs(*env.StopLoss-entry) * amount
		if slDist > 0 {
			rrVal := net / slDist
			rr = &rrVal
		}
		if env.TakeProfit != nil {
			tpDist := math.Abs(*env.TakeProfit - entry)
			slDistPrice := math.Abs(*env.StopLoss - entry)
			if slDistPrice > 0 {
				rrpVal := tpDist / slDistPrice
				rrPlanned = &rrpVal
			}
		}
	}

	pair := helpers.NormalizeSymbol(first.Symbol)

	return domain.Position{
		ID:         positionID,
		Side:       side,
		Pair:       pair,
		Amount:     helpers.Round8(amount),
		EntryPrice: helpers.Round8(entry),
		ExitPrice:  helpers.Round8(exit),
		Pnl:        helpers.Round8(pnl),
		NetPnl:     helpers.Round8(net),
		Commission: helpers.Round8(fee),
		Funding:    helpers.Round8(env.Funding),
		MAE:        mae,
		MFE:        mfe,
		TP:         env.TakeProfit,
		SL:         env.StopLoss,
		RR:         rr,
		RRPlanned:  rrPlanned,
		Isolated:   true,
		Closed:     true,
		Status:     &status,
		CreatedAt:  start,
		ClosedAt:   &end,
		Orders:     orders,
	}, nil
}
