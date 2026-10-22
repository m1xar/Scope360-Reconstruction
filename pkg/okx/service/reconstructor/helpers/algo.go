package helpers

import (
	"sort"
	"strings"

	"github.com/m1xar/scope360-reconstruction/pkg/okx/connector/okx/models"
)

func GroupAlgosByInst(algos []models.AlgoOrder) map[string][]models.AlgoOrder {
	idx := make(map[string][]models.AlgoOrder)
	for _, a := range algos {
		idx[a.InstId] = append(idx[a.InstId], a)
	}
	return idx
}

func AlgoLevels(algos []models.AlgoOrder, scope PositionScope) (tp, sl *float64) {
	posSide := strings.ToLower(scope.PosSide)
	mgnMode := strings.ToLower(scope.MgnMode)

	matched := make([]models.AlgoOrder, 0)
	for _, a := range algos {
		if a.InstId != scope.InstId {
			continue
		}
		algoPosSide := strings.ToLower(a.PosSide)
		if posSide != "" && posSide != "net" && algoPosSide != "" && algoPosSide != "net" && algoPosSide != posSide {
			continue
		}
		if algoTdMode := strings.ToLower(a.TdMode); mgnMode != "" && algoTdMode != "" && algoTdMode != mgnMode {
			continue
		}
		created := MustInt64(a.CTime)
		if created < scope.FromMs-boundarySlackMs {
			continue
		}
		if scope.ToMs > 0 && created > scope.ToMs {
			continue
		}
		matched = append(matched, a)
	}
	sort.SliceStable(matched, func(i, j int) bool {
		return MustInt64(matched[i].CTime) < MustInt64(matched[j].CTime)
	})

	for _, a := range matched {
		applyLevel(&tp, a.TpTriggerPx, a.TpOrdPx)
		applyLevel(&sl, a.SlTriggerPx, a.SlOrdPx)
		if tp != nil && sl != nil {
			break
		}
	}
	return tp, sl
}
