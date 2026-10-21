package helpers

import (
	"math"

	"github.com/m1xar/scope360-reconstruction/pkg/mexc/connector/mexc/models"
)

type OrderContext struct {
	Leverage   float64
	Isolated   bool
	TakeProfit *float64
	StopLoss   *float64
}

func ContextForEpisode(ep Episode) OrderContext {
	var ctx OrderContext
	opening := false
	for _, part := range ep.Parts {
		order := part.Order
		if !opening && part.Sign == ep.OpenSign {
			ctx.Leverage = float64(order.Leverage)
			ctx.Isolated = order.OpenType == 1
			opening = true
		}
		if order.StopLossPrice > 0 && ctx.StopLoss == nil {
			v := Round8(order.StopLossPrice)
			ctx.StopLoss = &v
		}
		if order.TakeProfitPrice > 0 && ctx.TakeProfit == nil {
			v := Round8(order.TakeProfitPrice)
			ctx.TakeProfit = &v
		}
	}
	return ctx
}

func AllocateFillStats(part FillPart, contractSizes map[string]float64) (fee, pnl float64) {
	ratio := 1.0
	if fullSize := ContractVolumeToBase(part.Order.DealVol, contractSizes[part.Order.Symbol]); fullSize > sizeEpsilon {
		ratio = part.Size / fullSize
		if ratio > 1 {
			ratio = 1
		}
	}
	fee = (math.Abs(part.Order.TakerFee) + math.Abs(part.Order.MakerFee)) * ratio
	pnl = part.Order.Profit * ratio
	return fee, pnl
}

func FundingForRange(records []models.FundingRecord, symbol string, positionType int, fromMs, toMs int64) float64 {
	var total float64
	for _, r := range records {
		if r.Symbol != symbol || (r.PositionType != 0 && r.PositionType != positionType) {
			continue
		}
		if r.SettleTime < fromMs || r.SettleTime > toMs {
			continue
		}
		total += r.Funding
	}
	return Round8(total)
}
