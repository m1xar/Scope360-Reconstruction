package helpers

import (
	"math"

	connector "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader"
	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

func Money(value int64, digits uint32) float64 {
	return float64(value) / math.Pow10(int(digits))
}

func MoneyDigits(session *connector.Session, deal *pb.ProtoOADeal) uint32 {
	if detail := deal.GetClosePositionDetail(); detail != nil && detail.MoneyDigits != nil {
		return detail.GetMoneyDigits()
	}
	if deal != nil && deal.MoneyDigits != nil {
		return deal.GetMoneyDigits()
	}
	if session != nil && session.MoneyDigits != 0 {
		return session.MoneyDigits
	}
	return 0
}

func Abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func Round8(v float64) float64 {
	return math.Round(v*1e8) / 1e8
}
