package builders

import (
	"strconv"

	"github.com/google/uuid"
	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
	"github.com/m1xar/scope360-reconstruction/pkg/ctrader/service/reconstructor/helpers"
	"github.com/m1xar/scope360-reconstruction/pkg/domain"
)

func BuildOpenPositions(
	res *pb.ProtoOAReconcileRes,
	symbols map[int64]string,
	currentPrices map[int64]float64,
	session *connector.Session,
) []domain.OpenPosition {
	if res == nil {
		return []domain.OpenPosition{}
	}
	ordersByPosition := make(map[int64][]*pb.ProtoOAOrder)
	for _, order := range res.GetOrder() {
		ordersByPosition[order.GetPositionId()] = append(ordersByPosition[order.GetPositionId()], order)
	}
	out := make([]domain.OpenPosition, 0, len(res.GetPosition()))
	for _, pos := range res.GetPosition() {
		tradeData := pos.GetTradeData()
		id := uuid.NewSHA1(uuid.NameSpaceOID, []byte("ctrader-position-"+strconv.FormatInt(pos.GetPositionId(), 10)))
		out = append(out, domain.OpenPosition{
			ID:           id,
			Pair:         helpers.SymbolName(symbols, tradeData.GetSymbolId()),
			Amount:       volumeToAmount(tradeData.GetVolume()),
			Side:         TradeSide(tradeData.GetTradeSide()),
			EntryPrice:   pos.GetPrice(),
			CurrentPrice: currentPrices[tradeData.GetSymbolId()],
			OpenTime:     helpers.TimeFromMillis(tradeData.GetOpenTimestamp()),
			Orders:       BuildOrders(ordersByPosition[pos.GetPositionId()], nil, id, session),
		})
	}
	return out
}
