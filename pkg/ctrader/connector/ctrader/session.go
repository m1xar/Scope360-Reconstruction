package ctrader

type Session struct {
	CtidTraderAccountID        int64
	AccountLogin               int64
	IsLive                     bool
	BrokerTitleShort           string
	MoneyDigits                uint32
	LeverageInCents            uint32
	TotalMarginCalculationType string
	MaxLeverage                uint32
	IsLimitedRisk              bool
}

func (s Session) Endpoint() string {
	if s.IsLive {
		return LiveEndpoint
	}
	return DemoEndpoint
}
