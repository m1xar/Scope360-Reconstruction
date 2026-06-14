package builders

import (
	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func BuildSwaps(deals []*pb.ProtoOADeal, symbols map[int64]string, session *connector.Session) []domain.UserSwap {
	out := make([]domain.UserSwap, 0)
	for _, deal := range deals {
		detail := deal.GetClosePositionDetail()
		if detail == nil || detail.GetSwap() == 0 {
			continue
		}
		out = append(out, domain.UserSwap{
			Pair:      helpers.SymbolName(symbols, deal.GetSymbolId()),
			Amount:    helpers.Round8(helpers.Money(detail.GetSwap(), helpers.MoneyDigits(session, deal))),
			CreatedAt: helpers.TimeFromMillis(deal.GetExecutionTimestamp()),
		})
	}
	return out
}
