package builders

import (
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
	"github.com/m1xar/scope360-reconstruction/pkg/mexc/service/reconstructor/helpers"
)

func AttachNotionalBalanceInit(positions *[]domain.Position) {
	if positions == nil {
		return
	}

	for i := range *positions {
		pos := &(*positions)[i]
		pos.BalanceInit = helpers.Round8(pos.Amount * pos.EntryPrice)
	}
}
