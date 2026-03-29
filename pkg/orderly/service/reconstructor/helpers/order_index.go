package helpers

import (
	"github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly/models"
)

func BuildOrderMap(orders []models.OrderlyOrder) map[int64]models.OrderlyOrder {
	m := make(map[int64]models.OrderlyOrder, len(orders))
	for _, o := range orders {
		m[o.OrderID] = o
	}
	return m
}

type AlgoOrderIndex map[string][]models.OrderlyAlgoOrder

func BuildAlgoOrderIndex(algoOrders []models.OrderlyAlgoOrder) AlgoOrderIndex {
	idx := make(AlgoOrderIndex)
	for _, o := range algoOrders {
		idx[o.Symbol] = append(idx[o.Symbol], o)
	}
	return idx
}

func ExtractTPSL(
	idx AlgoOrderIndex,
	symbol string,
	fromMs, toMs int64,
) (sl, tp *float64) {
	orders, ok := idx[symbol]
	if !ok {
		return nil, nil
	}

	for _, o := range orders {
		if o.CreatedTime < fromMs || o.CreatedTime > toMs {
			continue
		}

		if len(o.ChildOrders) > 0 {
			for _, child := range o.ChildOrders {
				extractFromAlgoOrder(&child, &sl, &tp)
			}
			if sl != nil || tp != nil {
				return sl, tp
			}
		}

		extractFromAlgoOrder(&o, &sl, &tp)
		if sl != nil && tp != nil {
			return sl, tp
		}
	}

	return sl, tp
}

func extractFromAlgoOrder(o *models.OrderlyAlgoOrder, sl, tp **float64) {
	if o.TriggerPrice == 0 {
		return
	}

	price := Round8(o.TriggerPrice)

	switch o.AlgoType {
	case "STOP":
		*sl = &price
	case "TPSL", "positional_TPSL", "TP_SL":

		if o.Side == "BUY" {

			*tp = &price
		} else {
			*sl = &price
		}
	}
}

func ExtractFunding(
	fundings []models.OrderlyFunding,
	symbol string,
	fromMs, toMs int64,
) float64 {
	total := 0.0

	for _, f := range fundings {
		if f.Symbol != symbol {
			continue
		}
		if f.CreatedTime < fromMs || f.CreatedTime > toMs {
			continue
		}

		total += f.FundingFee
	}

	return total
}
