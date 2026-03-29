package helpers

import "github.com/m1xar/Hyperliquid_Reconstruction/pkg/orderly/connector/orderly/models"

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
	const (
		beforeOpenToleranceMs = int64(120000)
		afterCloseToleranceMs = int64(60000)
	)

	orders, ok := idx[symbol]
	if !ok {
		return nil, nil
	}

	for _, o := range orders {
		if o.CreatedTime < fromMs-beforeOpenToleranceMs || o.CreatedTime > toMs+afterCloseToleranceMs {
			continue
		}
		if o.AlgoType != "TP_SL" {
			continue
		}

		for _, child := range o.ChildOrders {
			if child.TriggerPrice == 0 {
				continue
			}
			switch child.AlgoType {
			case "TAKE_PROFIT":
				if tp == nil {
					p := Round8(child.TriggerPrice)
					tp = &p
				}
			case "STOP_LOSS":
				if sl == nil {
					p := Round8(child.TriggerPrice)
					sl = &p
				}
			}
			if sl != nil && tp != nil {
				return sl, tp
			}
		}
	}

	return sl, tp
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
